package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go-chat/internal/pkg/protocol"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// 配置参数
var (
	// 基础配置
	apiHost    string
	wsHost     string
	userCount  int
	msgContent string
	timeout    time.Duration

	// 模式配置
	mode string // "burst" (洪峰) or "sustain" (持续)

	// Burst 模式参数
	msgCount int           // 每个用户发送多少条
	interval time.Duration // 发送间隔

	// Sustain 模式参数
	duration time.Duration // 压测持续时间
	minThink time.Duration // 最小思考时间
	maxThink time.Duration // 最大思考时间

	targetUsers []uint // 存储所有注册用户的ID
)

// 统计指标
type Stats struct {
	SentCount int64
	RecvCount int64
	ErrCount  int64
	Latencies []int64 // 毫秒
	Lock      sync.Mutex
	StartTime time.Time
	EndTime   time.Time
}

var stats = Stats{
	Latencies: make([]int64, 0, 100000),
}

func init() {
	flag.StringVar(&apiHost, "api", "http://localhost:8080", "API Server address")
	flag.StringVar(&wsHost, "ws", "ws://localhost:8080", "WebSocket Server address")
	flag.StringVar(&mode, "mode", "burst", "Benchmark mode: 'burst' (fixed count) or 'sustain' (fixed duration)")

	flag.IntVar(&userCount, "u", 50, "Number of concurrent users")
	flag.StringVar(&msgContent, "msg", "bench_msg", "Message content prefix")
	flag.DurationVar(&timeout, "t", 30*time.Second, "Timeout for reading responses")

	// Burst args
	flag.IntVar(&msgCount, "n", 20, "[Burst] Messages per user")
	flag.DurationVar(&interval, "i", 100*time.Millisecond, "[Burst] Fixed send interval")

	// Sustain args
	flag.DurationVar(&duration, "d", 60*time.Second, "[Sustain] Test duration")
	flag.DurationVar(&minThink, "min-think", 1*time.Second, "[Sustain] Min think time between msgs")
	flag.DurationVar(&maxThink, "max-think", 5*time.Second, "[Sustain] Max think time between msgs")

	flag.Parse()
}

type BenchUser struct {
	ID       uint
	Username string
	Password string
	Token     string
	Conn      *websocket.Conn
	FriendIDs []uint
	Lock      sync.Mutex
}

func main() {
	log.Println("==================================================")
	log.Println("      Go-Chat Benchmark Tool v2.0")
	log.Println("==================================================")
	log.Printf("Target:   API=%s, WS=%s", apiHost, wsHost)
	log.Printf("Users:    %d", userCount)
	log.Printf("Mode:     %s", mode)

	if mode == "burst" {
		log.Printf("Config:   %d msgs/user, interval=%v", msgCount, interval)
	} else {
		log.Printf("Config:   Duration=%v, ThinkTime=%v-%v", duration, minThink, maxThink)
	}
	log.Println("--------------------------------------------------")

	users := make([]*BenchUser, userCount)
	targetUsers = make([]uint, 0, userCount)

	// 1. 注册并登录所有用户
	log.Println("[Phase 1] Register and Login...")
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // 限制并发登录数

	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			u := &BenchUser{
				Username: fmt.Sprintf("bench_user_%d", idx),
				Password: "password123",
			}

			register(u)
			if err := loginAndGetID(u); err != nil {
				log.Printf("User %s login failed: %v", u.Username, err)
				atomic.AddInt64(&stats.ErrCount, 1)
				return
			}
			users[idx] = u
		}(i)
	}
	wg.Wait()

	// 过滤失败用户
	validUsers := make([]*BenchUser, 0, userCount)
	for _, u := range users {
		if u != nil && u.ID > 0 {
			validUsers = append(validUsers, u)
			targetUsers = append(targetUsers, u.ID)
		}
	}
	users = validUsers
	log.Printf(">> Active Users: %d", len(users))

	if len(users) < 2 {
		log.Fatal("Need at least 2 users to test P2P messaging")
	}

	// 2. 建立好友关系 (Ring Topology)
	log.Println("[Phase 1.5] Establishing Friendships (Ring Topology)...")
	setupRelations(users)

	// 3. 建立 WebSocket 连接
	log.Println("[Phase 2] Connect WebSockets...")
	for _, u := range users {
		wg.Add(1)
		go func(user *BenchUser) {
			defer wg.Done()
			if err := connectWS(user); err != nil {
				log.Printf("User %s WS connect failed: %v", user.Username, err)
				atomic.AddInt64(&stats.ErrCount, 1)
			}
		}(u)
	}
	wg.Wait()
	log.Println(">> All WebSockets connected.")

	// 4. 开始压测
	log.Println("[Phase 3] Running Benchmark...")
	stats.StartTime = time.Now()

	// 启动接收协程
	for _, u := range users {
		if u.Conn != nil {
			go readLoop(u)
		}
	}

	// 启动发送协程
	sendWg := sync.WaitGroup{}
	for _, u := range users {
		if u.Conn != nil {
			sendWg.Add(1)
			go func(user *BenchUser) {
				defer sendWg.Done()
				if mode == "burst" {
					runBurst(user)
				} else {
					runSustain(user)
				}
			}(u)
		}
	}

	sendWg.Wait()
	sendDuration := time.Since(stats.StartTime)
	log.Printf(">> Sending finished in %v. Waiting %v for lingering messages...", sendDuration, timeout)

	// 等待一段时间让消息到达
	time.Sleep(5 * time.Second)
	stats.EndTime = time.Now()

	// 5. 输出报告
	printReport(sendDuration)
}

func runBurst(u *BenchUser) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	hbTicker := time.NewTicker(30 * time.Second)
	defer hbTicker.Stop()

	for i := 0; i < msgCount; {
		select {
		case <-hbTicker.C:
			u.Conn.WriteJSON(protocol.Message{
				Type:    protocol.TypeHeartbeat,
				Content: "ping",
			})
		case <-ticker.C:
			sendMsg(u, i)
			i++
		}
	}
}

func runSustain(u *BenchUser) {
	endTime := time.Now().Add(duration)
	msgIdx := 0

	hbTicker := time.NewTicker(30 * time.Second)
	defer hbTicker.Stop()

	for time.Now().Before(endTime) {
		select {
		case <-hbTicker.C:
			u.Conn.WriteJSON(protocol.Message{
				Type:    protocol.TypeHeartbeat,
				Content: "ping",
			})
		default:
			// 随机思考时间
			thinkTime := minThink + time.Duration(rand.Int63n(int64(maxThink-minThink)))
			time.Sleep(thinkTime)

			sendMsg(u, msgIdx)
			msgIdx++
		}
	}
}

func sendMsg(u *BenchUser, seq int) {
	if len(u.FriendIDs) == 0 {
		return
	}

	// 随机选择一个好友
	targetID := u.FriendIDs[rand.Intn(len(u.FriendIDs))]

	payload := BenchMsgPayload{
		Timestamp: time.Now().UnixMilli(),
		Content:   fmt.Sprintf("%s-%d-%d", msgContent, u.ID, seq),
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := protocol.Message{
		Type:     protocol.TypeSingleMsg,
		TargetID: targetID,
		Content:  string(payloadBytes),
	}

	if err := u.Conn.WriteJSON(msg); err != nil {
		atomic.AddInt64(&stats.ErrCount, 1)
		return
	}
	atomic.AddInt64(&stats.SentCount, 1)
}

func readLoop(u *BenchUser) {
	defer func() {
		if u.Conn != nil {
			u.Conn.Close()
		}
	}()

	for {
		u.Conn.SetReadDeadline(time.Now().Add(timeout))
		_, message, err := u.Conn.ReadMessage()
		if err != nil {
			return
		}

		var reply protocol.Reply
		if err := json.Unmarshal(message, &reply); err != nil {
			continue
		}

		if reply.Type != protocol.TypeSingleMsg {
			continue
		}

		now := time.Now().UnixMilli()
		atomic.AddInt64(&stats.RecvCount, 1)

		var payload BenchMsgPayload
		if err := json.Unmarshal([]byte(reply.Content), &payload); err == nil && payload.Timestamp > 0 {
			latency := now - payload.Timestamp
			if latency >= 0 {
				stats.Lock.Lock()
				stats.Latencies = append(stats.Latencies, latency)
				stats.Lock.Unlock()
			}
		}
	}
}

func printReport(duration time.Duration) {
	log.Println("==================================================")
	log.Println("            Benchmark Report")
	log.Println("==================================================")

	sent := atomic.LoadInt64(&stats.SentCount)
	recv := atomic.LoadInt64(&stats.RecvCount)
	errs := atomic.LoadInt64(&stats.ErrCount)

	// QPS Calculation
	qpsSend := float64(sent) / duration.Seconds()
	qpsRecv := float64(recv) / duration.Seconds() // Note: Recv duration is technically longer

	log.Printf("Time Elapsed:  %v", duration)
	log.Printf("Total Sent:    %d", sent)
	log.Printf("Total Recv:    %d", recv)
	log.Printf("Total Errors:  %d", errs)
	log.Printf("Loss Rate:     %.2f%%", (1.0-float64(recv)/float64(sent))*100)
	log.Println("--------------------------------------------------")
	log.Printf("QPS (Send):    %.2f req/s", qpsSend)
	log.Printf("QPS (Recv):    %.2f req/s", qpsRecv)
	log.Println("--------------------------------------------------")
	log.Println("Latency Distribution (End-to-End):")

	stats.Lock.Lock()
	latencies := make([]int64, len(stats.Latencies))
	copy(latencies, stats.Latencies)
	stats.Lock.Unlock()

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		var sum int64
		for _, l := range latencies {
			sum += l
		}
		avg := float64(sum) / float64(len(latencies))

		log.Printf("  Avg:   %.2f ms", avg)
		log.Printf("  Min:   %d ms", latencies[0])
		log.Printf("  Max:   %d ms", latencies[len(latencies)-1])
		log.Printf("  P50:   %d ms", latencies[int(float64(len(latencies))*0.50)])
		log.Printf("  P95:   %d ms", latencies[int(float64(len(latencies))*0.95)])
		log.Printf("  P99:   %d ms", latencies[int(float64(len(latencies))*0.99)])
	} else {
		log.Println("  No latency data collected.")
	}
	log.Println("==================================================")
}

// ---------------- Helper Functions (Same as before) ----------------

func register(u *BenchUser) {
	url := fmt.Sprintf("%s/api/user/register", apiHost)
	body := map[string]string{
		"username": u.Username,
		"password": u.Password,
		"email":    fmt.Sprintf("%s@example.com", u.Username),
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func loginAndGetID(u *BenchUser) error {
	// 1. Login
	url := fmt.Sprintf("%s/api/user/login", apiHost)
	body := map[string]string{
		"username": u.Username,
		"password": u.Password,
	}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var loginRes struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &loginRes); err != nil {
		return err
	}
	if loginRes.Code != 0 {
		return fmt.Errorf("login api code %d", loginRes.Code)
	}
	u.Token = loginRes.Data.Token

	// 2. Get User Info (to get ID)
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/user/info", apiHost), nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)
	client := &http.Client{}
	infoResp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer infoResp.Body.Close()

	infoBytes, _ := io.ReadAll(infoResp.Body)
	var infoRes struct {
		Code int `json:"code"`
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(infoBytes, &infoRes); err != nil {
		return err
	}
	u.ID = infoRes.Data.ID
	return nil
}

func connectWS(u *BenchUser) error {
	uStr := fmt.Sprintf("%s/api/ws?token=%s", wsHost, u.Token)
	conn, _, err := websocket.DefaultDialer.Dial(uStr, nil)
	if err != nil {
		return err
	}
	u.Conn = conn
	return nil
}

type BenchMsgPayload struct {
	Timestamp int64  `json:"ts"`
	Content   string `json:"content"`
}

// ---------------- Friendship Helpers ----------------

func setupRelations(users []*BenchUser) {
	var wg sync.WaitGroup
	// Limit concurrency for API calls
	sem := make(chan struct{}, 20)

	// 1. Load existing friends
	log.Println("Loading existing friends...")
	for _, u := range users {
		wg.Add(1)
		go func(user *BenchUser) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := loadFriends(user); err != nil {
				log.Printf("Failed to load friends for %s: %v", user.Username, err)
			}
		}(u)
	}
	wg.Wait()

	// 2. Ensure Ring Topology
	log.Println("Ensuring ring topology...")
	count := len(users)
	for i := 0; i < count; i++ {
		u1 := users[i]
		u2 := users[(i+1)%count]

		// Check if u2 is already a friend of u1
		isFriend := false
		u1.Lock.Lock()
		for _, fid := range u1.FriendIDs {
			if fid == u2.ID {
				isFriend = true
				break
			}
		}
		u1.Lock.Unlock()

		if !isFriend {
			wg.Add(1)
			go func(u1, u2 *BenchUser) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if err := makeFriends(u1, u2); err != nil {
					log.Printf("Failed to make friends %s <-> %s: %v", u1.Username, u2.Username, err)
				}
			}(u1, u2)
		}
	}
	wg.Wait()
}

func loadFriends(u *BenchUser) error {
	url := fmt.Sprintf("%s/api/friend/list", apiHost)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+u.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var res struct {
		Code int `json:"code"`
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("api code %d", res.Code)
	}

	u.Lock.Lock()
	u.FriendIDs = make([]uint, 0, len(res.Data))
	for _, f := range res.Data {
		u.FriendIDs = append(u.FriendIDs, f.ID)
	}
	u.Lock.Unlock()
	return nil
}

func makeFriends(u1, u2 *BenchUser) error {
	// 1. u1 sends request to u2
	err := sendFriendRequest(u1, u2.ID)
	if err != nil {
		// If already friend, we can just update local cache and return
		if err.Error() == "already_friend" {
			u1.Lock.Lock()
			u1.FriendIDs = append(u1.FriendIDs, u2.ID)
			u1.Lock.Unlock()
			
			u2.Lock.Lock()
			u2.FriendIDs = append(u2.FriendIDs, u1.ID)
			u2.Lock.Unlock()
			return nil
		}
		return fmt.Errorf("send request failed: %w", err)
	}

	// 2. u2 gets pending requests
	reqID, err := getFriendRequestID(u2, u1.ID)
	if err != nil {
		return fmt.Errorf("get request failed: %w", err)
	}

	// 3. u2 accepts request
	if err := handleFriendRequest(u2, reqID, 1); err != nil {
		return fmt.Errorf("accept request failed: %w", err)
	}

	// Update local cache
	u1.Lock.Lock()
	u1.FriendIDs = append(u1.FriendIDs, u2.ID)
	u1.Lock.Unlock()

	u2.Lock.Lock()
	u2.FriendIDs = append(u2.FriendIDs, u1.ID)
	u2.Lock.Unlock()
	return nil
}

func sendFriendRequest(u *BenchUser, targetID uint) error {
	url := fmt.Sprintf("%s/api/friend/request", apiHost)
	body := map[string]interface{}{
		"target_id": targetID,
		"remark":    "bench_add",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+u.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return err
	}

	if res.Code != 0 {
		if res.Msg == "你们已经是好友了" {
			return fmt.Errorf("already_friend")
		}
		if res.Msg == "已发送过申请，请等待对方处理" {
			return nil // Proceed to accept
		}
		return fmt.Errorf("api code %d: %s", res.Code, res.Msg)
	}
	return nil
}

func getFriendRequestID(u *BenchUser, senderID uint) (uint, error) {
	// Retry a few times because of async processing or latency
	for i := 0; i < 10; i++ {
		url := fmt.Sprintf("%s/api/friend/requests", apiHost)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+u.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var res struct {
			Code int `json:"code"`
			Data []struct {
				ID       uint `json:"id"`
				SenderID uint `json:"sender_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(bodyBytes, &res); err != nil {
			return 0, err
		}

		for _, r := range res.Data {
			if r.SenderID == senderID {
				return r.ID, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("request not found")
}

func handleFriendRequest(u *BenchUser, reqID uint, action int) error {
	url := fmt.Sprintf("%s/api/friend/handle", apiHost)
	body := map[string]interface{}{
		"request_id": reqID,
		"action":     action,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+u.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
