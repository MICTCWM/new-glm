package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	ticketUploadRoot   = "ticket_images"
	ticketMaxFileSize  = 10 * 1024 * 1024 // 10MB
	ticketMaxImageName = 255
)

var allowedImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".gif": true, ".webp": true,
}

func isAllowedImageExt(ext string) bool {
	return allowedImageExts[strings.ToLower(ext)]
}

// sanitizeImageFilename returns a safe filename using only the extension from user input
func sanitizeImageFilename(fname string, timestamp int64) (string, error) {
	ext := strings.ToLower(filepath.Ext(fname))
	if !isAllowedImageExt(ext) {
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
	return fmt.Sprintf("%d%s", timestamp, ext), nil
}

// nolint
// collectTicketUserIds gathers all user IDs from tickets and replies
func collectTicketUserIds(tickets []model.Ticket) []int {
	idSet := make(map[int]bool)
	for i := range tickets {
		idSet[tickets[i].UserId] = true
		for j := range tickets[i].Replies {
			idSet[tickets[i].Replies[j].UserId] = true
		}
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids
}

// populateTicketUsernames batch-fetches usernames from user IDs and fills them into tickets
func populateTicketUsernames(tickets []model.Ticket) {
	ids := collectTicketUserIds(tickets)
	if len(ids) == 0 {
		return
	}
	var users []model.User
	if err := model.DB.Select("id, username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to batch-fetch usernames for tickets: %v", err))
		return
	}
	usernameMap := make(map[int]string, len(users))
	for _, u := range users {
		usernameMap[u.Id] = u.Username
	}
	for i := range tickets {
		tickets[i].Username = usernameMap[tickets[i].UserId]
		for j := range tickets[i].Replies {
			tickets[i].Replies[j].Username = usernameMap[tickets[i].Replies[j].UserId]
		}
	}
}

// CreateTicket handles ticket creation with optional image uploads
func CreateTicket(c *gin.Context) {
	userId := c.GetInt("id")
	title := strings.TrimSpace(c.PostForm("title"))
	content := strings.TrimSpace(c.PostForm("content"))

	if title == "" || content == "" {
		common.ApiError(c, fmt.Errorf("title and content are required"))
		return
	}
	if len(title) > 255 {
		common.ApiError(c, fmt.Errorf("title must not exceed 255 characters"))
		return
	}

	ticket := model.Ticket{
		UserId:  userId,
		Title:   title,
		Content: content,
		Status:  model.TicketStatusOpen,
	}

	if err := model.DB.Create(&ticket).Error; err != nil {
		common.ApiError(c, fmt.Errorf("failed to create ticket: %v", err))
		return
	}

	// Handle uploaded images
	form, err := c.MultipartForm()
	if err == nil {
		files := form.File["images"]
		uploadDir := filepath.Join(ticketUploadRoot, strconv.Itoa(ticket.Id))
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			common.ApiError(c, fmt.Errorf("failed to create upload directory: %v", err))
			return
		}

		for _, file := range files {
			// Validate file size
			if file.Size > ticketMaxFileSize {
				continue
			}

			// Sanitize filename — only use the extension
			timestamp := time.Now().UnixNano()
			savedName, err := sanitizeImageFilename(file.Filename, timestamp)
			if err != nil {
				continue
			}
			savePath := filepath.Join(uploadDir, savedName)

			src, err := file.Open()
			if err != nil {
				continue
			}

			dst, err := os.Create(savePath)
			if err != nil {
				src.Close()
				continue
			}

			if _, err := io.Copy(dst, src); err == nil {
				filePath := "/uploads/" + filepath.ToSlash(savePath)
				ticketImage := model.TicketImage{
					TicketId: ticket.Id,
					Filename: filepath.Base(file.Filename),
					FilePath: filePath,
					FileSize: file.Size,
				}
				if err := model.DB.Create(&ticketImage).Error; err != nil {
					common.SysLog(fmt.Sprintf("failed to save ticket %d image record: %v", ticket.Id, err))
				} else {
					ticket.Images = append(ticket.Images, ticketImage)
				}
			}

			dst.Close()
			src.Close()
		}
	}

	common.ApiSuccess(c, ticket)
}

// GetUserTickets returns tickets for the authenticated user
func GetUserTickets(c *gin.Context) {
	userId := c.GetInt("id")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []model.Ticket
	var total int64

	query := model.DB.Model(&model.Ticket{}).Where("user_id = ?", userId)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Preload("Images").Preload("Replies").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&tickets)

	// Batch-fetch usernames
	populateTicketUsernames(tickets)

	if tickets == nil {
		tickets = []model.Ticket{}
	}

	common.ApiSuccess(c, gin.H{
		"tickets":   tickets,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetAllTickets returns all tickets (admin only)
func GetAllTickets(c *gin.Context) {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tickets []model.Ticket
	var total int64

	query := model.DB.Model(&model.Ticket{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Preload("Images").Preload("Replies").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&tickets)

	// Batch-fetch usernames
	populateTicketUsernames(tickets)

	if tickets == nil {
		tickets = []model.Ticket{}
	}

	common.ApiSuccess(c, gin.H{
		"tickets":   tickets,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTicketDetail returns a single ticket with images and replies
func GetTicketDetail(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid ticket id"))
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")

	var ticket model.Ticket
	if err := model.DB.Preload("Images").Preload("Replies").First(&ticket, ticketId).Error; err != nil {
		common.ApiError(c, fmt.Errorf("ticket not found"))
		return
	}

	// Only ticket owner or admin can view
	if ticket.UserId != userId && role < common.RoleAdminUser {
		common.ApiError(c, fmt.Errorf("permission denied"))
		return
	}

	// Batch-fetch usernames
	populateTicketUsernames([]model.Ticket{ticket})

	common.ApiSuccess(c, ticket)
}

// AddTicketReply adds a reply to a ticket (admin only)
func AddTicketReply(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid ticket id"))
		return
	}

	userId := c.GetInt("id")

	var req struct {
		Content      string `json:"content"`
		CloseOnReply bool   `json:"close_on_reply"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, fmt.Errorf("invalid request body"))
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		common.ApiError(c, fmt.Errorf("reply content is required"))
		return
	}

	var ticket model.Ticket
	if err := model.DB.First(&ticket, ticketId).Error; err != nil {
		common.ApiError(c, fmt.Errorf("ticket not found"))
		return
	}

	reply := model.TicketReply{
		TicketId: ticketId,
		UserId:   userId,
		Content:  content,
	}

	if err := model.DB.Create(&reply).Error; err != nil {
		common.ApiError(c, fmt.Errorf("failed to create reply: %v", err))
		return
	}

	// Update ticket status
	newStatus := model.TicketStatusReplied
	if req.CloseOnReply {
		newStatus = model.TicketStatusClosed
	}
	if err := model.DB.Model(&ticket).Updates(map[string]interface{}{
		"status": newStatus,
	}).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to update ticket %d status after reply: %v", ticketId, err))
	}

	// Populate username
	var user model.User
	if err := model.DB.Select("username").First(&user, reply.UserId).Error; err == nil {
		reply.Username = user.Username
	}

	common.ApiSuccess(c, reply)
}

// UpdateTicketStatus updates a ticket's status (admin only)
func UpdateTicketStatus(c *gin.Context) {
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid ticket id"))
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, fmt.Errorf("invalid request body"))
		return
	}

	if req.Status != model.TicketStatusOpen && req.Status != model.TicketStatusClosed && req.Status != model.TicketStatusReplied {
		common.ApiError(c, fmt.Errorf("invalid status: must be 'open', 'closed', or 'replied'"))
		return
	}

	var ticket model.Ticket
	if err := model.DB.First(&ticket, ticketId).Error; err != nil {
		common.ApiError(c, fmt.Errorf("ticket not found"))
		return
	}

	if err := model.DB.Model(&ticket).Update("status", req.Status).Error; err != nil {
		common.ApiError(c, fmt.Errorf("failed to update ticket status: %v", err))
		return
	}

	common.ApiSuccess(c, gin.H{"id": ticketId, "status": req.Status})
}

// UploadTicketImage handles standalone image upload for ticket editing
func UploadTicketImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiError(c, fmt.Errorf("no file uploaded"))
		return
	}

	// Validate file size
	if file.Size > ticketMaxFileSize {
		common.ApiError(c, fmt.Errorf("file size exceeds maximum allowed 10MB"))
		return
	}

	// Validate file type
	timestamp := time.Now().UnixNano()
	savedName, err := sanitizeImageFilename(file.Filename, timestamp)
	if err != nil {
		common.ApiError(c, fmt.Errorf("unsupported file type: %v", err))
		return
	}

	uploadDir := filepath.Join(ticketUploadRoot, "temp")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		common.ApiError(c, fmt.Errorf("failed to create upload directory: %v", err))
		return
	}

	savePath := filepath.Join(uploadDir, savedName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		common.ApiError(c, fmt.Errorf("failed to save file: %v", err))
		return
	}

	common.ApiSuccess(c, gin.H{
		"filename": filepath.Base(file.Filename),
		"filepath": "/uploads/" + filepath.ToSlash(savePath),
		"filesize": file.Size,
	})
}

// ServeTicketImage serves uploaded ticket images behind authentication,
// only allowing safe image extensions and preventing path traversal.
func ServeTicketImage(c *gin.Context) {
	reqPath := c.Param("filepath")

	// Only allow safe image extensions
	ext := strings.ToLower(path.Ext(reqPath))
	if !isAllowedImageExt(ext) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Prevent path traversal by resolving and comparing absolute paths
	absBase, _ := filepath.Abs(ticketUploadRoot)
	fullPath := filepath.Join(ticketUploadRoot, reqPath)
	absTarget, err := filepath.Abs(fullPath)
	if err != nil || !strings.HasPrefix(absTarget, absBase) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.File(fullPath)
}
