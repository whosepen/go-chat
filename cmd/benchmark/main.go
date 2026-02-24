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
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// 配置参数
var (
	apiHost     string
	wsHost      string
	userCount   int
	msgCount    int
	interval    time.Duration
	timeout     time.Duration
	msgContent  string
	targetUsers []uint // 存储所有注册用户的ID
)

// 统计指标
var (
	sentCount     int64
	recvCount     int64
	errCount      int64
	totalLatency  int64 // 累计延迟 (ms)
	latencyCounts int64 // 延迟统计次数
)

func init() {
	flag.StringVar(&apiHost, "api", "http://localhost:8080", "API Server address")
	flag.StringVar(&wsHost, "ws", "ws://localhost:8080", "WebSocket Server address")
	flag.IntVar(&userCount, "u", 50, "Number of concurrent users")
	flag.IntVar(&msgCount, "n", 20, "Messages per user")
	flag.DurationVar(&interval, "i", 100*time.Millisecond, "Send interval")
	flag.DurationVar(&timeout, "t", 30*time.Second, "Timeout duration")
	flag.StringVar(&msgContent, "msg", "bench_msg", "Message content prefix")
	flag.Parse()
}

type BenchUser struct {
	ID       uint
	Username string
	Password string
	Token    string
	Conn     *websocket.Conn
}

func main() {
	log.Println("Starting Benchmark...")
	log.Printf("Target: %s (API), %s (WS)\n", apiHost, wsHost)
	log.Printf("Users: %d, Msgs/User: %d, Interval: %v\n", userCount, msgCount, interval)

	users := make([]*BenchUser, userCount)
	targetUsers = make([]uint, 0, userCount)

	// 1. 注册并登录所有用户
	log.Println("Phase 1: Register and Login...")
	var wg sync.WaitGroup
	// 限制并发注册，避免瞬间压垮数据库
	sem := make(chan struct{}, 10)

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

			// 注册 (忽略错误，可能已存在)
			register(u)

			// 登录并获取ID
			if err := loginAndGetID(u); err != nil {
				log.Printf("User %s login failed: %v", u.Username, err)
				atomic.AddInt64(&errCount, 1)
				return
			}

			users[idx] = u
		}(i)
	}
	wg.Wait()

	// 收集成功登录的用户ID
	validUsers := make([]*BenchUser, 0, userCount)
	for _, u := range users {
		if u != nil && u.ID > 0 {
			validUsers = append(validUsers, u)
			targetUsers = append(targetUsers, u.ID)
		}
	}
	users = validUsers
	log.Printf("Successfully logged in %d users", len(users))

	if len(users) < 2 {
		log.Fatal("Need at least 2 users to test P2P messaging")
	}

	// 2. 建立 WebSocket 连接
	log.Println("Phase 2: Connect WebSockets...")
	for _, u := range users {
		wg.Add(1)
		go func(user *BenchUser) {
			defer wg.Done()
			if err := connectWS(user); err != nil {
				log.Printf("User %s WS connect failed: %v", user.Username, err)
				atomic.AddInt64(&errCount, 1)
			}
		}(u)
	}
	wg.Wait()
	log.Println("All WebSockets connected.")

	// 3. 开始压测
	log.Println("Phase 3: Sending Messages...")
	start := time.Now()

	// 启动接收协程 (不计入 WaitGroup，它们一直运行直到超时或程序结束)
	for _, u := range users {
		if u.Conn != nil {
			go readLoop(u)
		}
	}

	// 启动发送协程
	for _, u := range users {
		if u.Conn != nil {
			wg.Add(1)
			go func(user *BenchUser) {
				defer wg.Done()
				writeLoop(user)
			}(u)
		}
	}

	wg.Wait() // 等待所有发送完成
	sendDuration := time.Since(start)
	log.Printf("All messages sent in %v. Waiting %v for responses...", sendDuration, timeout)

	// 等待一段时间让消息到达接收方
	time.Sleep(5 * time.Second)

	// 4. 统计结果
	log.Println("--------------------------------------------------")
	log.Printf("Benchmark Report")
	log.Printf("--------------------------------------------------")
	log.Printf("Total Sent:   %d", atomic.LoadInt64(&sentCount))
	log.Printf("Total Recv:   %d", atomic.LoadInt64(&recvCount))
	log.Printf("Total Errors: %d", atomic.LoadInt64(&errCount))

	tps := float64(atomic.LoadInt64(&sentCount)) / sendDuration.Seconds()
	log.Printf("TPS (Send):   %.2f msg/s", tps)

	avgLatency := float64(0)
	count := atomic.LoadInt64(&latencyCounts)
	if count > 0 {
		avgLatency = float64(atomic.LoadInt64(&totalLatency)) / float64(count)
	}
	log.Printf("Avg Latency:  %.2f ms", avgLatency)
	log.Println("--------------------------------------------------")
}

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
	// 路径是 /ws?token=xxx (根据你的 router.go 配置，路径是 /ws 而不是 /api/ws)
	// 如果 router.go 里 protectGroup.GET("/ws", chatApi.Connect) 是在 /api 组下，则路径是 /api/ws
	// 检查代码： protectGroup := apiGroup.Group("") -> protectGroup.GET("/ws")
	// 所以路径确实是 /api/ws
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

func writeLoop(u *BenchUser) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for i := 0; i < msgCount; i++ {
		<-ticker.C

		// 随机选择接收者 (不选自己)
		targetID := u.ID
		for targetID == u.ID {
			targetID = targetUsers[rand.Intn(len(targetUsers))]
		}

		// 构造消息内容，包含时间戳用于计算延迟
		payload := BenchMsgPayload{
			Timestamp: time.Now().UnixMilli(),
			Content:   fmt.Sprintf("%s-%d-%d", msgContent, u.ID, i),
		}
		payloadBytes, _ := json.Marshal(payload)

		msg := protocol.Message{
			Type:     protocol.TypeSingleMsg,
			TargetID: targetID,
			Content:  string(payloadBytes),
		}

		err := u.Conn.WriteJSON(msg)
		if err != nil {
			log.Printf("User %d send error: %v", u.ID, err)
			atomic.AddInt64(&errCount, 1)
			return
		}
		atomic.AddInt64(&sentCount, 1)
	}
}

func readLoop(u *BenchUser) {
	defer func() {
		if u.Conn != nil {
			u.Conn.Close()
		}
	}()

	for {
		// 设置读取超时
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

		atomic.AddInt64(&recvCount, 1)

		// 计算延迟
		var payload BenchMsgPayload
		if err := json.Unmarshal([]byte(reply.Content), &payload); err == nil && payload.Timestamp > 0 {
			latency := time.Now().UnixMilli() - payload.Timestamp
			if latency > 0 {
				atomic.AddInt64(&totalLatency, latency)
				atomic.AddInt64(&latencyCounts, 1)
			}
		}
	}
}
