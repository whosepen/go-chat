package api

import (
	"go-chat/internal/pkg/utils"
	"go-chat/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateGroup 创建群组
// @Summary 创建群组
// @Description 创建一个新的群聊，创建者自动成为群主
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.CreateGroupReq true "创建群组参数"
// @Success 200 {object} utils.Response{data=service.GroupInfoDTO}
// @Router /group/create [post]
func CreateGroup(c *gin.Context) {
	var req service.CreateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	group, err := service.CreateGroup(c.Request.Context(), userID, req)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.Success(c, group)
}

// GetGroupInfo 获取群信息
// @Summary 获取群组信息
// @Description 根据群ID获取群组详细信息
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param group_id query uint true "群组ID"
// @Success 200 {object} utils.Response{data=service.GroupInfoDTO}
// @Router /group/info [get]
func GetGroupInfo(c *gin.Context) {
	groupIDStr := c.Query("group_id")
	if groupIDStr == "" {
		utils.FailWithCode(c, http.StatusBadRequest, "请提供群组ID")
		return
	}

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "群组ID格式错误")
		return
	}

	userID := c.GetUint("userID")
	group, err := service.GetGroupInfo(c.Request.Context(), uint(groupID), userID)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.Success(c, group)
}

// GetGroupMembers 获取群成员列表
// @Summary 获取群成员列表
// @Description 获取指定群组的成员列表
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param group_id query uint true "群组ID"
// @Success 200 {object} utils.Response{data=[]service.GroupMemberDTO}
// @Router /group/members [get]
func GetGroupMembers(c *gin.Context) {
	groupIDStr := c.Query("group_id")
	if groupIDStr == "" {
		utils.FailWithCode(c, http.StatusBadRequest, "请提供群组ID")
		return
	}

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "群组ID格式错误")
		return
	}

	userID := c.GetUint("userID")
	members, err := service.GetGroupMembers(c.Request.Context(), uint(groupID), userID)
	if err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.Success(c, members)
}

// SendGroupRequest 发送入群申请
// @Summary 发送入群申请
// @Description 向指定群组发送入群申请
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.SendGroupRequestReq true "入群申请参数"
// @Success 200 {object} utils.Response
// @Router /group/join [post]
func SendGroupRequest(c *gin.Context) {
	var req service.SendGroupRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.JoinGroup(c.Request.Context(), int(userID), req); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "入群申请已发送", nil)
}

// HandleGroupRequest 处理入群申请
// @Summary 处理入群申请
// @Description 同意或拒绝收到的入群申请（仅群主和管理员可操作）
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.HandleGroupRequestReq true "处理申请参数"
// @Success 200 {object} utils.Response
// @Router /group/handle-join [post]
func HandleGroupRequest(c *gin.Context) {
	var req service.HandleGroupRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.HandleGroupRequest(c.Request.Context(), userID, req); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	if req.Action == 1 {
		utils.SuccessWithMsg(c, "已同意入群申请", nil)
	} else {
		utils.SuccessWithMsg(c, "已拒绝入群申请", nil)
	}
}

// GetMyGroups 获取我的群聊列表
// @Summary 获取我的群聊列表
// @Description 获取当前用户加入的所有群组，返回包含未读消息数和最新消息时间
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} utils.Response{data=[]service.GroupListReqDTO}
// @Router /group/my-groups [get]
func GetMyGroups(c *gin.Context) {
	userID := c.GetUint("userID")

	groups, err := service.GetMyGroups(c.Request.Context(), userID)
	if err != nil {
		utils.ServerError(c, "获取群聊列表失败")
		return
	}

	utils.Success(c, groups)
}

// QuitGroup 退出群聊
// @Summary 退出群聊
// @Description 退出指定的群聊（群主不能直接退出，需先转让或解散）
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.QuitGroupReq true "退出群组参数"
// @Success 200 {object} utils.Response
// @Router /group/quit [post]
func QuitGroup(c *gin.Context) {
	var req service.QuitGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.QuitGroup(c.Request.Context(), userID, req.GroupID); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "已退出群聊", nil)
}

// UpdateGroupInfo 修改群信息
// @Summary 修改群信息
// @Description 修改群组名称、头像、描述（仅群主可操作）
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.UpdateGroupInfoReq true "修改群信息参数"
// @Success 200 {object} utils.Response
// @Router /group/info [put]
func UpdateGroupInfo(c *gin.Context) {
	var req service.UpdateGroupInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID := c.GetUint("userID")

	if err := service.UpdateGroupInfo(c.Request.Context(), userID, req); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "群信息已更新", nil)
}

// KickMember 踢出群成员
// @Summary 踢出群成员
// @Description 将指定成员踢出群聊（群主和管理员可操作）
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body service.KickMemberReq true "踢人参数"
// @Success 200 {object} utils.Response
// @Router /group/kick [post]
func KickMember(c *gin.Context) {
	var req service.KickMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.FailWithCode(c, http.StatusBadRequest, "参数错误")
		return
	}

	operatorID := c.GetUint("userID")

	if err := service.KickMember(c.Request.Context(), operatorID, req.UserID, req.GroupID); err != nil {
		utils.Fail(c, err.Error())
		return
	}

	utils.SuccessWithMsg(c, "已将成员移出群聊", nil)
}

// GetGroupRequests 获取收到的入群申请列表
// @Summary 获取收到的入群申请
// @Description 获取当前用户作为群主或管理员收到的待处理入群申请
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Success 200 {object} utils.Response{data=[]service.GroupRequestDTO}
// @Router /group/requests [get]
func GetGroupRequests(c *gin.Context) {
	userID := c.GetUint("userID")

	requests, err := service.GetPendingGroupRequests(c.Request.Context(), userID)
	if err != nil {
		utils.ServerError(c, "获取申请列表失败")
		return
	}

	utils.Success(c, requests)
}

// SearchGroupByCode 通过code查找群组
// @Summary 通过群号搜索群组
// @Description 根据群号（展示用ID）搜索群组信息
// @Tags 群组模块
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param group_code query string true "群号"
// @Success 200 {object} utils.Response{data=service.GroupInfoDTO}
// @Router /group/search [get]
func SearchGroupByCode(c *gin.Context) {
	groupCode := c.Query("group_code")

	if groupCode == "" {
		utils.Fail(c, "group code不能为空")
		return
	}

	res, err := service.SearchGroupByCode(c.Request.Context(), groupCode)
	if err != nil {
		utils.ServerError(c, "未找到群组")
		return
	}

	utils.Success(c, res)
}

// 标记群聊信息已读
func MarkGroupMessagesAsRead(c *gin.Context) {
	var req service.MarkGroupMessagesAsReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	userID := c.GetUint("userID")
	if err := service.MarkGroupMessagesAsRead(c.Request.Context(), userID, req.TargetID); err != nil {
		utils.ServerError(c, "标记失败")
		return
	}
	utils.SuccessWithMsg(c, "标记成功", nil)
}

func UpdateGroupMemberInfo(c *gin.Context) {
	var req service.UpdateGroupMemberInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	userID := c.GetUint("userID")
	if err := service.UpdateGroupMemberInfo(c.Request.Context(), userID, req); err != nil {
		utils.ServerError(c, "修改失败")
		return
	}
	utils.SuccessWithMsg(c, "修改成功", nil)
}
