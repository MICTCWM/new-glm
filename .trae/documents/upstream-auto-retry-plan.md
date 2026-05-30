# 上游请求自动重试功能实施计划

## 需求概述

当检测到上游返回任何类型的请求错误时，系统自动代替用户重新请求上游（同一渠道），最多重试 5 次。重试过程中不中断与用户的连接。如果 5 次内某次请求成功，则返回成功结果；如果 5 次全部失败，则将第 5 次的错误信息返回给用户。同时在日志中记录重试次数。

---

## 1. 新增配置项：`UpstreamRetryTimes`

### 文件：`common/constants.go`
- 在第 153 行 `var RetryTimes = 0` 下方新增：
```go
var UpstreamRetryTimes = 5
```

### 文件：`model/option.go`
- 在 `OptionMap` 注册处（约第 159 行）添加：
```go
common.OptionMap["UpstreamRetryTimes"] = strconv.Itoa(common.UpstreamRetryTimes)
```
- 在 `case` 匹配处（约第 496 行，`case "RetryTimes"` 附近）添加：
```go
case "UpstreamRetryTimes":
    common.UpstreamRetryTimes, _ = strconv.Atoi(value)
```

---

## 2. 在 `RelayInfo` 中新增重试计数字段

### 文件：`relay/common/relay_info.go`
在 `RelayInfo` 结构体中（约第 151 行 `LastError` 字段附近）新增：
```go
UpstreamRetryCount int // 上游请求重试次数
```

---

## 3. 改造各 Handler 函数 —— 添加上游自动重试循环

### 通用重试模式

对于**非流式请求**，在 `DoRequest` → 状态码检查 → `DoResponse` → `PostConsume` 整个链路外层包裹重试循环：

```go
upstreamRetryTimes := common.UpstreamRetryTimes
var httpResp *http.Response
var lastApiErr *types.NewAPIError

for attempt := 0; attempt <= upstreamRetryTimes; attempt++ {
    // 非首次尝试时，重新构造 requestBody
    var reqBody io.Reader
    if attempt == 0 {
        reqBody = requestBody
    } else {
        // 从原始 jsonData 或 BodyStorage 重新创建
        reqBody = recreateRequestBody(c, info, jsonData)
    }

    resp, err := adaptor.DoRequest(c, info, reqBody)
    if err != nil {
        lastApiErr = types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
        if attempt >= upstreamRetryTimes {
            return lastApiErr
        }
        info.UpstreamRetryCount = attempt + 1
        continue
    }

    if resp != nil {
        httpResp = resp.(*http.Response)
        if httpResp.StatusCode != http.StatusOK {
            httpResp.Body.Close()
            lastApiErr = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
            service.ResetStatusCode(lastApiErr, statusCodeMappingStr)
            if attempt >= upstreamRetryTimes {
                return lastApiErr
            }
            info.UpstreamRetryCount = attempt + 1
            continue
        }
    }

    // 对于流式请求，DoResponse 开始后无法重试，直接跳出循环
    if info.IsStream {
        break
    }

    usage, apiErr := adaptor.DoResponse(c, httpResp, info)
    if apiErr != nil {
        service.ResetStatusCode(apiErr, statusCodeMappingStr)
        lastApiErr = apiErr
        if attempt >= upstreamRetryTimes {
            return lastApiErr
        }
        info.UpstreamRetryCount = attempt + 1
        continue
    }

    // 成功
    info.UpstreamRetryCount = attempt
    // PostConsume & return nil
    // ...
}

// 流式请求：在重试循环外调用 DoResponse
if info.IsStream {
    usage, apiErr := adaptor.DoResponse(c, httpResp, info)
    if apiErr != nil {
        service.ResetStatusCode(apiErr, statusCodeMappingStr)
        return apiErr
    }
    // PostConsume & return nil
    // ...
}
```

### 3.1 TextHelper — `relay/compatible_handler.go`

改造 `TextHelper` 函数（第 26-216 行），将 `DoRequest` → 状态码检查 → `DoResponse` → `PostConsume` 部分包裹到重试循环中。

**关键点**：
- 在 passThrough 模式下，使用 `common.GetBodyStorage(c)` 并在重试时 `Seek(0, io.SeekStart)`
- 在非 passThrough 模式下，保留 `jsonData` 变量，重试时用 `bytes.NewBuffer(jsonData)` 重新创建
- `info.IsStream` 可能在 `DoRequest` 后才确定（通过 `Content-Type` 头），需要动态判断

### 3.2 ImageHelper — `relay/image_handler.go`

改造 `ImageHelper` 函数（第 23-157 行），同 TextHelper 模式。

### 3.3 EmbeddingHelper — `relay/embedding_handler.go`

改造 `EmbeddingHelper` 函数（第 20-88 行），同 TextHelper 模式。

### 3.4 AudioHelper — `relay/audio_handler.go`

改造 `AudioHelper` 函数（第 18-77 行）。

**关键点**：Audio 请求的 `requestBody` 来自 `adaptor.ConvertAudioRequest`，对于 multipart form 需要特殊处理。由于音频 TTS/STT 请求的 body 可能无法重新读取，对于 passThrough 模式使用 `BodyStorage`，非 passThrough 模式需要保留 `ioReader`。对于 multipart form 类型的请求（STT），重试时需要重新调用 `adaptor.ConvertAudioRequest`。

**注意**：AudioHelper 中 `DoRequest` 已调用 `adaptor.ConvertAudioRequest` 获取 `ioReader`。对于 TTS（`RelayModeAudioSpeech`），body 是 JSON，可以重新构造；对于 STT（transcription/translation），body 是 multipart form，可能无法重放。对此类请求，仅对 passThrough 模式支持重试（因为 `BodyStorage` 支持 Seek），非 passThrough 模式由于 multipart form 的复杂性，暂不支持重试。

### 3.5 RerankHelper — `relay/rerank_handler.go`

改造 `RerankHelper` 函数（第 20-101 行），同 TextHelper 模式（rerank 总是非流式）。

### 3.6 ClaudeHelper — `relay/claude_handler.go`

改造 `ClaudeHelper` 函数（第 24-214 行），同 TextHelper 模式。

### 3.7 GeminiHelper — `relay/gemini_handler.go`

改造 `GeminiHelper` 函数（第 55-199 行），同 TextHelper 模式。

### 3.8 GeminiEmbeddingHandler — `relay/gemini_handler.go`

改造 `GeminiEmbeddingHandler` 函数（第 201-293 行），同 TextHelper 模式。

### 3.9 ResponsesHelper — `relay/responses_handler.go`

改造 `ResponsesHelper` 函数（第 23-161 行），同 TextHelper 模式。

---

## 4. 日志中记录重试次数

### 4.1 消费日志（成功时）

在 `service/text_quota.go` 的 `PostTextConsumeQuota` 函数中，在构建 `other` map 后、调用 `model.RecordConsumeLog` 前，添加：

```go
if relayInfo.UpstreamRetryCount > 0 {
    other["upstream_retry_count"] = relayInfo.UpstreamRetryCount
}
```

同样在以下位置添加：
- `service/quota.go` 的 `PostAudioConsumeQuota` 函数
- `service/quota.go` 的 `PostWssConsumeQuota` 函数

### 4.2 错误日志（失败时）

在 `controller/relay.go` 的 `processChannelError` 函数（约第 356 行）中，记录错误日志的 `other` map 中添加：

```go
if relayInfo.UpstreamRetryCount > 0 {
    other["upstream_retry_count"] = relayInfo.UpstreamRetryCount
}
```

### 4.3 前端展示

前端的日志详情页面 (`web/default/` 和 `web/classic/`) 中，日志列表/详情的 `Other` 字段如果包含 `upstream_retry_count`，则在日志信息旁边显示 `已重试 N 次`。

**具体修改**：
- `web/default/src/` 中日志展示组件（`features/logs/` 或类似路径），在渲染日志详情时检查 `other.upstream_retry_count` 并展示
- `web/classic/src/` 同理

---

## 5. 实施步骤

### 步骤 1：添加配置项（文件：`common/constants.go`, `model/option.go`）
- [ ] `common/constants.go:153` 后添加 `var UpstreamRetryTimes = 5`
- [ ] `model/option.go:159` 添加 OptionMap 注册
- [ ] `model/option.go:496` 添加 case 分支

### 步骤 2：RelayInfo 添加字段（文件：`relay/common/relay_info.go`）
- [ ] 在 `RelayInfo` 结构体中添加 `UpstreamRetryCount int`

### 步骤 3：改造 TextHelper（文件：`relay/compatible_handler.go`）
- [ ] 将 DoRequest → 状态检查 → DoResponse 包裹到重试循环
- [ ] 区分流式/非流式处理
- [ ] 重试时正确重新构造 requestBody

### 步骤 4：改造 ImageHelper（文件：`relay/image_handler.go`）
- [ ] 同上

### 步骤 5：改造 EmbeddingHelper（文件：`relay/embedding_handler.go`）
- [ ] 同上

### 步骤 6：改造 AudioHelper（文件：`relay/audio_handler.go`）
- [ ] 同上，注意 multipart form 限制

### 步骤 7：改造 RerankHelper（文件：`relay/rerank_handler.go`）
- [ ] 同上

### 步骤 8：改造 ClaudeHelper（文件：`relay/claude_handler.go`）
- [ ] 同上

### 步骤 9：改造 GeminiHelper（文件：`relay/gemini_handler.go`）
- [ ] 同上

### 步骤 10：改造 GeminiEmbeddingHandler（文件：`relay/gemini_handler.go`）
- [ ] 同上

### 步骤 11：改造 ResponsesHelper（文件：`relay/responses_handler.go`）
- [ ] 同上

### 步骤 12：消费日志添加重试次数（文件：`service/text_quota.go`, `service/quota.go`）
- [ ] `PostTextConsumeQuota`: 在 `other` map 中添加 `upstream_retry_count`
- [ ] `PostAudioConsumeQuota`: 同上
- [ ] `PostWssConsumeQuota`: 同上

### 步骤 13：错误日志添加重试次数（文件：`controller/relay.go`）
- [ ] `processChannelError`: 在 `other` map 中添加 `upstream_retry_count`

### 步骤 14：前端展示重试次数
- [ ] `web/default/` 日志组件添加重试次数展示
- [ ] `web/classic/` 日志组件添加重试次数展示

### 步骤 15：测试验证
- [ ] 编译通过 `go build ./...`
- [ ] 单元测试（如有相关测试）
- [ ] 手动测试非流式请求自动重试
- [ ] 手动测试流式请求自动重试
- [ ] 验证日志中正确记录重试次数

---

## 6. 注意事项

1. **流式请求限制**：流式请求仅在 `DoRequest` 返回错误或状态码非 200 时可重试。一旦 `DoResponse` 开始流式传输数据给客户端，中途出现错误无法撤销已发送的数据，因此不支持重试。

2. **Audio multipart 限制**：音频 STT（transcription/translation）请求的 body 是 multipart form，非 passThrough 模式下不便重新构造，暂不支持该场景的重试。

3. **与现有 `RetryTimes` 的区别**：
   - `RetryTimes`：渠道级别的重试，失败后切换到不同渠道
   - `UpstreamRetryTimes`：上游级别的重试，失败后使用同一渠道重新请求

4. **响应体关闭**：每次重试前需确保上一次的 `http.Response.Body` 已关闭，避免连接泄漏。

5. **计费安全**：预扣费仅在首次请求前执行一次（在 `controller/relay.go` 的 `Relay` 函数中），重试不重复扣费。成功后通过 `SettleBilling` 结算。失败后通过 `Refund` 退费。