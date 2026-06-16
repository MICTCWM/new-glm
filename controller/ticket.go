package controller

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// CreateTicket handles ticket creation with optional image uploads
func CreateTicket(c *gin.Context) {
	userId := c.GetInt("id")
	title := strings.TrimSpace(c.PostForm("title"))
	content := strings.TrimSpace(c.PostForm("content"))

	if title == "" || content == "" {
		common.ApiError(c, fmt.Errorf("title and content are required"))
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
		uploadDir := filepath.Join("ticket_images", strconv.Itoa(ticket.Id))
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			common.ApiError(c, fmt.Errorf("failed to create upload directory: %v", err))
			return
		}

		for _, file := range files {
			timestamp := time.Now().UnixNano()
			ext := filepath.Ext(file.Filename)
			savedName := fmt.Sprintf("%d_%s%s", timestamp, strings.TrimSuffix(file.Filename, ext), ext)
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
				ticketImage := model.TicketImage{
					TicketId: ticket.Id,
					Filename: file.Filename,
					FilePath: "/uploads/" + filepath.ToSlash(savePath),
					FileSize: file.Size,
				}
				model.DB.Create(&ticketImage)
				ticket.Images = append(ticket.Images, ticketImage)
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

	// Populate username for each ticket
	for i := range tickets {
		var user model.User
		if err := model.DB.Select("username").First(&user, tickets[i].UserId).Error; err == nil {
			tickets[i].Username = user.Username
		}
		for j := range tickets[i].Replies {
			var replyUser model.User
			if err := model.DB.Select("username").First(&replyUser, tickets[i].Replies[j].UserId).Error; err == nil {
				tickets[i].Replies[j].Username = replyUser.Username
			}
		}
	}

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

	// Populate username for each ticket
	for i := range tickets {
		var user model.User
		if err := model.DB.Select("username").First(&user, tickets[i].UserId).Error; err == nil {
			tickets[i].Username = user.Username
		}
		for j := range tickets[i].Replies {
			var replyUser model.User
			if err := model.DB.Select("username").First(&replyUser, tickets[i].Replies[j].UserId).Error; err == nil {
				tickets[i].Replies[j].Username = replyUser.Username
			}
		}
	}

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

	// Populate username
	var user model.User
	if err := model.DB.Select("username").First(&user, ticket.UserId).Error; err == nil {
		ticket.Username = user.Username
	}
	for j := range ticket.Replies {
		var replyUser model.User
		if err := model.DB.Select("username").First(&replyUser, ticket.Replies[j].UserId).Error; err == nil {
			ticket.Replies[j].Username = replyUser.Username
		}
	}

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
		Content        string `json:"content"`
		CloseOnReply   bool   `json:"close_on_reply"`
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
	model.DB.Model(&ticket).Updates(map[string]interface{}{
		"status": newStatus,
	})

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

	model.DB.Model(&ticket).Update("status", req.Status)

	common.ApiSuccess(c, gin.H{"id": ticketId, "status": req.Status})
}

// UploadTicketImage handles standalone image upload for ticket editing
func UploadTicketImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiError(c, fmt.Errorf("no file uploaded"))
		return
	}

	uploadDir := "ticket_images/temp"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		common.ApiError(c, fmt.Errorf("failed to create upload directory: %v", err))
		return
	}

	timestamp := time.Now().UnixNano()
	ext := filepath.Ext(file.Filename)
	savedName := fmt.Sprintf("%d_%s%s", timestamp, strings.TrimSuffix(file.Filename, ext), ext)
	savePath := filepath.Join(uploadDir, savedName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		common.ApiError(c, fmt.Errorf("failed to save file: %v", err))
		return
	}

	common.ApiSuccess(c, gin.H{
		"filename": file.Filename,
		"filepath": "/uploads/" + filepath.ToSlash(savePath),
		"filesize": file.Size,
	})
}
