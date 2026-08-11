package service

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// 请求日志通知（Request Log Notify）
//
// 在管理员不在线时，定时（或按日/按周）把一段时间内的请求数据汇总成邮件，
// 发送给管理员配置的邮箱。三种汇总模式：
//  1. 周期汇报：每隔 N 小时汇报一次（上一次汇报时间 ~ 本次汇报时间）。
//  2. 天总结：每天固定小时发送一份“昨日/当日”汇总。
//  3. 周总结：每周固定星期几的固定小时发送一份“上周”汇总。
//
// 注意：Log 表未记录“首字延迟(TTFT)”，因此“平均请求首字”目前无法精确获取，
// 邮件中以“平均请求耗时(秒)”与“平均输出速度(tokens/s)”近似呈现。
// 若后续需要在日志中记录首字延迟，可在 model.Log 增加字段并在 relay 层写入。

// 配置项 Key（统一存于 OptionMap）
const (
	OptRequestLogNotifyEnabled          = "RequestLogNotifyEnabled"
	OptRequestLogNotifyEmails           = "RequestLogNotifyEmails"
	OptRequestLogNotifyIntervalHours    = "RequestLogNotifyIntervalHours"
	OptRequestLogNotifyDailyEnabled     = "RequestLogNotifyDailyEnabled"
	OptRequestLogNotifyDailyHour        = "RequestLogNotifyDailyHour"
	OptRequestLogNotifyWeeklyEnabled    = "RequestLogNotifyWeeklyEnabled"
	OptRequestLogNotifyWeeklyDay        = "RequestLogNotifyWeeklyDay"
	OptRequestLogNotifyWeeklyHour       = "RequestLogNotifyWeeklyHour"
	OptRequestLogNotifyLastPeriodicTime = "RequestLogNotifyLastPeriodicTime"
	OptRequestLogNotifyLastDailyDate    = "RequestLogNotifyLastDailyDate"
	OptRequestLogNotifyLastWeeklyDate   = "RequestLogNotifyLastWeeklyDate"
)

func requestLogNotifyGetBool(key string) bool {
	return requestLogNotifyGetValue(key) == "true"
}

func requestLogNotifyGetInt(key string, def int) int {
	v := requestLogNotifyGetValue(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return def
	}
	return n
}

func requestLogNotifyGetEmails() []string {
	raw := strings.TrimSpace(requestLogNotifyGetValue(OptRequestLogNotifyEmails))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func requestLogNotifyGetValue(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key]
}

// requestLogWindowStats 是某一时段的统计结果
type requestLogWindowStats struct {
	Start time.Time
	End   time.Time

	TotalRequests  int64
	RPMAvg         float64
	TPMAvg         float64
	MaxUseTime     float64
	MinUseTime     float64
	AvgUseTime     float64
	AvgOutputSpeed float64 // tokens/s
	Users          []string
	Models         []requestLogModelStat
	ErrorCount     int64
	ErrorKinds     int
	ErrorBreakdown []requestLogErrorStat
}

type requestLogModelStat struct {
	Model  string
	Count  int64
	Tokens int64
}

type requestLogErrorStat struct {
	Code  int
	Count int64
}

// aggregateRequestLogs 在 [start, end) 时间窗内聚合请求日志统计
func aggregateRequestLogs(start, end time.Time) (*requestLogWindowStats, error) {
	stats := &requestLogWindowStats{Start: start, End: end}

	// 基础指标：总数、总耗时、总 token、错误数
	type baseAgg struct {
		Total      int64
		SumUseTime float64
		MaxUseTime float64
		MinUseTime float64
		PromptTok  int64
		CompletTok int64
		ErrorCount int64
	}
	var base baseAgg
	err := model.DB.Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Select("COUNT(*) AS total, " +
			"COALESCE(SUM(use_time),0) AS sum_use_time, " +
			"COALESCE(MAX(use_time),0) AS max_use_time, " +
			"COALESCE(MIN(use_time),0) AS min_use_time, " +
			"COALESCE(SUM(prompt_tokens),0) AS prompt_tok, " +
			"COALESCE(SUM(completion_tokens),0) AS complet_tok, " +
			"COALESCE(SUM(CASE WHEN type = 'error' THEN 1 ELSE 0 END),0) AS error_count").
		Scan(&base).Error
	if err != nil {
		return nil, err
	}

	stats.TotalRequests = base.Total
	stats.MaxUseTime = base.MaxUseTime
	stats.MinUseTime = base.MinUseTime
	stats.ErrorCount = base.ErrorCount

	minutes := end.Sub(start).Minutes()
	if minutes <= 0 {
		minutes = 1
	}
	stats.RPMAvg = float64(base.Total) / minutes
	totalTokens := base.PromptTok + base.CompletTok
	stats.TPMAvg = float64(totalTokens) / minutes
	stats.AvgUseTime = 0
	if base.Total > 0 {
		stats.AvgUseTime = base.SumUseTime / float64(base.Total)
	}
	stats.AvgOutputSpeed = 0
	if base.SumUseTime > 0 {
		stats.AvgOutputSpeed = float64(base.CompletTok) / base.SumUseTime
	}

	// 模型维度
	type modelAgg struct {
		ModelName string
		Cnt       int64
		Tok       int64
	}
	var modelRows []modelAgg
	err = model.DB.Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ?", start, end).
		Select("model_name AS model_name, COUNT(*) AS cnt, COALESCE(SUM(prompt_tokens)+SUM(completion_tokens),0) AS tok").
		Group("model_name").
		Order("cnt DESC").
		Scan(&modelRows).Error
	if err != nil {
		return nil, err
	}
	for _, m := range modelRows {
		if m.ModelName == "" {
			m.ModelName = "(unknown)"
		}
		stats.Models = append(stats.Models, requestLogModelStat{Model: m.ModelName, Count: m.Cnt, Tokens: m.Tok})
	}

	// 用户维度
	type userRow struct {
		Username string
	}
	var userRows []userRow
	err = model.DB.Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ? AND username <> ''", start, end).
		Select("DISTINCT username").
		Scan(&userRows).Error
	if err != nil {
		return nil, err
	}
	for _, u := range userRows {
		stats.Users = append(stats.Users, u.Username)
	}
	sort.Strings(stats.Users)

	// 错误维度（按 code 聚合）
	type errAgg struct {
		Code int
		Cnt  int64
	}
	var errRows []errAgg
	err = model.DB.Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ? AND type = 'error'", start, end).
		Select("code AS code, COUNT(*) AS cnt").
		Group("code").
		Order("cnt DESC").
		Scan(&errRows).Error
	if err != nil {
		return nil, err
	}
	for _, e := range errRows {
		stats.ErrorBreakdown = append(stats.ErrorBreakdown, requestLogErrorStat{Code: e.Code, Count: e.Cnt})
	}
	stats.ErrorKinds = len(stats.ErrorBreakdown)

	return stats, nil
}

func (s *requestLogWindowStats) toHTML(title string) string {
	var b strings.Builder
	b.WriteString("<div style='font-family:Helvetica,Arial,sans-serif;color:#222;max-width:720px'>")
	b.WriteString(fmt.Sprintf("<h2 style='margin:0 0 4px'>%s</h2>", html.EscapeString(title)))
	b.WriteString(fmt.Sprintf("<p style='color:#666;margin:0 0 16px'>统计时段：%s ~ %s</p>",
		s.Start.Format("2006-01-02 15:04:05"), s.End.Format("2006-01-02 15:04:05")))

	b.WriteString("<table style='border-collapse:collapse;width:100%;font-size:13px'>")
	row := func(k, v string) {
		b.WriteString(fmt.Sprintf("<tr><td style='padding:6px 10px;border:1px solid #eee;color:#666'>%s</td><td style='padding:6px 10px;border:1px solid #eee;font-weight:600'>%s</td></tr>", k, v))
	}
	row("总请求数", fmt.Sprintf("%d", s.TotalRequests))
	row("平均 RPM", fmt.Sprintf("%.2f", s.RPMAvg))
	row("平均 TPM", fmt.Sprintf("%.2f", s.TPMAvg))
	row("最长请求耗时", fmt.Sprintf("%.2f s", s.MaxUseTime))
	row("最短请求耗时", fmt.Sprintf("%.2f s", s.MinUseTime))
	row("平均请求耗时(近似首字/总耗时)", fmt.Sprintf("%.2f s", s.AvgUseTime))
	row("平均输出速度", fmt.Sprintf("%.2f tokens/s", s.AvgOutputSpeed))
	b.WriteString("</table>")

	// 模型
	b.WriteString("<h3 style='margin:18px 0 6px'>请求模型及用量</h3>")
	if len(s.Models) == 0 {
		b.WriteString("<p style='color:#999'>本时段无请求</p>")
	} else {
		b.WriteString("<table style='border-collapse:collapse;width:100%;font-size:13px'>")
		b.WriteString("<tr style='background:#fafafa'><th style='padding:6px 10px;border:1px solid #eee;text-align:left'>模型</th><th style='padding:6px 10px;border:1px solid #eee;text-align:right'>请求数</th><th style='padding:6px 10px;border:1px solid #eee;text-align:right'>Tokens</th></tr>")
		for _, m := range s.Models {
			b.WriteString(fmt.Sprintf("<tr><td style='padding:6px 10px;border:1px solid #eee'>%s</td><td style='padding:6px 10px;border:1px solid #eee;text-align:right'>%d</td><td style='padding:6px 10px;border:1px solid #eee;text-align:right'>%d</td></tr>", html.EscapeString(m.Model), m.Count, m.Tokens))
		}
		b.WriteString("</table>")
	}

	// 用户
	b.WriteString("<h3 style='margin:18px 0 6px'>请求用户</h3>")
	if len(s.Users) == 0 {
		b.WriteString("<p style='color:#999'>无</p>")
	} else {
		escapedUsers := make([]string, 0, len(s.Users))
		for _, user := range s.Users {
			escapedUsers = append(escapedUsers, html.EscapeString(user))
		}
		b.WriteString(fmt.Sprintf("<p style='font-size:13px'>共 %d 位：%s</p>", len(s.Users), strings.Join(escapedUsers, "、")))
	}

	// 错误
	b.WriteString("<h3 style='margin:18px 0 6px'>错误情况</h3>")
	b.WriteString(fmt.Sprintf("<p style='font-size:13px'>错误次数：<b>%d</b>；错误种类：<b>%d</b></p>", s.ErrorCount, s.ErrorKinds))
	if len(s.ErrorBreakdown) > 0 {
		b.WriteString("<table style='border-collapse:collapse;width:100%;font-size:13px'>")
		b.WriteString("<tr style='background:#fafafa'><th style='padding:6px 10px;border:1px solid #eee;text-align:left'>错误码</th><th style='padding:6px 10px;border:1px solid #eee;text-align:right'>次数</th></tr>")
		for _, e := range s.ErrorBreakdown {
			b.WriteString(fmt.Sprintf("<tr><td style='padding:6px 10px;border:1px solid #eee'>%d</td><td style='padding:6px 10px;border:1px solid #eee;text-align:right'>%d</td></tr>", e.Code, e.Count))
		}
		b.WriteString("</table>")
	}

	b.WriteString("</div>")
	return b.String()
}

func requestLogNotifySend(title string, stats *requestLogWindowStats) bool {
	emails := requestLogNotifyGetEmails()
	if len(emails) == 0 {
		common.SysLog("RequestLogNotify: 未配置收件邮箱，跳过发送")
		return false
	}
	content := stats.toHTML(title)
	success := true
	for _, email := range emails {
		if err := common.SendEmail(title, email, content); err != nil {
			success = false
			common.SysError("RequestLogNotify: 发送邮件失败 -> " + email + ": " + err.Error())
		}
	}
	return success
}

// StartRequestLogNotifyTask 启动请求日志通知后台任务
func StartRequestLogNotifyTask() {
	go func() {
		common.SysLog("RequestLogNotify task started")
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if !common.IsMasterNode {
				continue
			}
			if !requestLogNotifyGetBool(OptRequestLogNotifyEnabled) {
				continue
			}
			now := time.Now()

			// 1) 周期汇报
			intervalHours := requestLogNotifyGetInt(OptRequestLogNotifyIntervalHours, 1)
			if intervalHours < 1 {
				intervalHours = 1
			}
			lastPeriodic := parseStoredTime(requestLogNotifyGetValue(OptRequestLogNotifyLastPeriodicTime), now.Add(-time.Duration(intervalHours)*time.Hour))
			if now.Sub(lastPeriodic) >= time.Duration(intervalHours)*time.Hour {
				stats, err := aggregateRequestLogs(lastPeriodic, now)
				if err != nil {
					common.SysError("RequestLogNotify: 周期统计失败: " + err.Error())
				} else if requestLogNotifySend(fmt.Sprintf("[请求日志] 周期汇报（近 %d 小时）", intervalHours), stats) {
					_ = model.UpdateOption(OptRequestLogNotifyLastPeriodicTime, now.Format(time.RFC3339))
				}
			}

			// 2) 天总结：到达配置 hour 且当日尚未发送时，补发昨天整段统计
			if requestLogNotifyGetBool(OptRequestLogNotifyDailyEnabled) {
				dailyHour := requestLogNotifyGetInt(OptRequestLogNotifyDailyHour, 9)
				todayKey := now.Format("2006-01-02")
				if now.Hour() >= dailyHour && requestLogNotifyGetValue(OptRequestLogNotifyLastDailyDate) != todayKey {
					yesterday := now.AddDate(0, 0, -1)
					start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
					end := start.AddDate(0, 0, 1)
					stats, err := aggregateRequestLogs(start, end)
					if err != nil {
						common.SysError("RequestLogNotify: 天总结统计失败: " + err.Error())
					} else if requestLogNotifySend(fmt.Sprintf("[请求日志] 天总结（%s）", start.Format("2006-01-02")), stats) {
						_ = model.UpdateOption(OptRequestLogNotifyLastDailyDate, todayKey)
					}
				}
			}

			// 3) 周总结：到达配置 weekday/hour 且本周尚未发送时，补发过去 7 天统计
			if requestLogNotifyGetBool(OptRequestLogNotifyWeeklyEnabled) {
				weeklyDay := requestLogNotifyGetInt(OptRequestLogNotifyWeeklyDay, 1) // 0=周日
				weeklyHour := requestLogNotifyGetInt(OptRequestLogNotifyWeeklyHour, 9)
				weekKey := now.Format("2006-01-02")
				if int(now.Weekday()) == weeklyDay && now.Hour() >= weeklyHour &&
					requestLogNotifyGetValue(OptRequestLogNotifyLastWeeklyDate) != weekKey {
					end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
					start := end.AddDate(0, 0, -7)
					stats, err := aggregateRequestLogs(start, end)
					if err != nil {
						common.SysError("RequestLogNotify: 周总结统计失败: " + err.Error())
					} else if requestLogNotifySend(fmt.Sprintf("[请求日志] 周总结（%s ~ %s）", start.Format("2006-01-02"), end.Add(-time.Second).Format("2006-01-02")), stats) {
						_ = model.UpdateOption(OptRequestLogNotifyLastWeeklyDate, weekKey)
					}
				}
			}
		}
	}()
}

func parseStoredTime(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return def
	}
	return t
}
