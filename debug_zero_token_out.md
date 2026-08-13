# 排查文档：流式请求客户端显示 "zero token out"（无 token 返回）

> 生成时间：2026-08-13
> 用途：带到服务器上按步骤排查。所有结论来自对当前代码（main 分支 + 未提交改动）的静态分析。

---

## 一、问题定义（已确认的事实）

| 项目 | 情况 |
|---|---|
| 请求类型 | 流式 stream=true（SSE） |
| 客户端表现 | 内容正常输出，但显示 "zero token out"（usage 无 token / 0 token） |
| new-api 后台请求日志 | 同一请求记录**有真实 token**（prompt/completion 都有值） |
| 影响范围 | 只有**部分**用户/请求 |

**核心矛盾**：客户端收到的流里没有（或为 0）token 数据，但后台日志计费用的是真实 token。
这说明：**客户端看到的 usage 和日志计费用的 usage 不是同一份数据。**

---

## 二、代码层结论（已排查，重要）

### 关键代码路径

流式 OpenAI 请求在 `relay/channel/openai/relay-openai.go` 的 `OaiStreamHandler` 中处理：

1. 上游每个 chunk 到达后**原样立即转发**给客户端（`HandleStreamFormat` → `sendStreamData`）；
2. 流结束后，`handleLastResponse` 从**最后一个 chunk** 提取 usage（`containStreamUsage`）；
3. 若最后一个 chunk 没有 usage，则用累积的响应文本估算 usage（`ResponseText2Usage`，真实值）；
4. **日志/计费用的是第 2/3 步计算出的 `usage` 对象；而客户端看到的是第 1 步转发的原始 chunk。**

### 嫌疑点 1（最可疑）：`include_usage=false` 时 usage chunk 被丢弃

`relay/channel/openai/relay-openai.go` 约 175-182 行：

```go
// OpenAI 格式且客户端未请求 usage（stream_options.include_usage=false）时，
// 丢弃仅含 usage 的 chunk，与 handleLastResponse 的 shouldSendLastResp 逻辑一致。
if info.RelayFormat == types.RelayFormatOpenAI &&
    !info.ShouldIncludeUsage &&
    streamDataIsUsageOnly(info.RelayMode, data) {
    return
}
```

- `ShouldIncludeUsage` 在 `relay/compatible_handler.go` 中设置：默认 `true`，**只有客户端显式发送 `stream_options` 且 `include_usage=false`（或发送空对象 `stream_options:{}`，Go 零值会解析成 false）时才是 false**。
- 一旦为 false：最后的 usage chunk 被丢弃 → 客户端只收到 content + 一个生成的 stop chunk，**完全没有 usage** → 客户端显示 "zero token out"；
- 同时 `handleLastResponse` 仍从最后 chunk 提取到 usage 用于计费 → **后台日志有真实 token**。
- **完全符合已确认的全部事实**，"部分用户" = 部分客户端软件会发 `include_usage:false` 或空 `stream_options`。

**验证方法**（服务器上）：
- 抓一条出问题请求的原始请求体，看 `stream_options` 字段是否缺失/为空/false。

### 嫌疑点 2：Claude 上游 → OpenAI 客户端，转换时 usage 丢失

`relay/channel/claude/relay-claude.go` 的 `StreamResponseClaude2OpenAI`（约 467 行）：
- `message_delta` 分支里 `//claudeUsage = &claudeResponse.Usage` 是**被注释掉的** —— 转换后的 OpenAI chunk **不带 usage 字段**；
- 但计费时 `relay/channel/openai/helper.go` 的 `handleClaudeFormat` 会从原始 chunk 提取 `info.ClaudeConvertInfo.Usage` → 真实 token；
- 结果：走 Claude 上游渠道的流式请求，客户端拿不到 usage，日志却是真实 token。

**验证方法**（服务器上）：
- 看出问题请求日志里的渠道类型（channel type），是否为 Claude 上游；
- 用 curl 直接请求该渠道复现，观察流末尾 chunk 是否有 usage。

### 嫌疑点 3：zero-output 重试误判

`relay/channel/openai/relay-openai.go` 约 255-258 行：

```go
lastStreamDataHasOutput := streamDataHasOutput(info.RelayMode, lastStreamData)
if !streamOutputSent && !lastStreamDataHasOutput && relaycommon.ShouldRetryZeroOutputUsageAfterStream(info, usage) {
    return nil, relaycommon.NewZeroOutputRetryError(info, usage)
}
```

- 触发条件：整个流**没有任何输出 chunk** 且最后 chunk 无输出、usage 里 input>0 且 output==0；
- 触发后返回错误 `"upstream returned zero output tokens, input_tokens=N"`（错误码 `ErrorCodeChannelZeroOutputTokens`），外层会重试；
- 若第一次尝试其实**有内容**但没被识别为"输出"（或客户端在第一次响应后才断开），重试成功时日志记录第二次的 token，而客户端可能拿到第一次的响应 → 显示 zero token out；
- 注意：`StreamScannerHandler` 中 `len(data)<6` 的数据行会被跳过（`relay/helper/stream_scanner.go` 约 258 行），`data:` 前缀缺失的裸 JSON 行也会被丢弃 —— 若某上游发送格式不规范，内容可能被静默丢弃。

---

## 三、服务器上请按顺序执行

### 第 1 步：找出问题请求的完整信息

- new-api 后台 → 日志页 → 筛选出问题的请求，记录：
  - `channel_id` / 渠道类型
  - 模型名、分组
  - 日志中 token 值（prompt/completion）
  - 日志 `other` 字段里是否有异常标记（如 `usage_semantic`、`input_tokens_total`、`retry`）

### 第 2 步：用 curl 复现并抓流末尾 chunk

```bash
# 用出问题的 token + 模型复现（注意流式、控制台直接看最后几个 data:）
curl -N -s -X POST http://127.0.0.1:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-你的令牌" \
  -H "Content-Type: application/json" \
  -d '{"model":"出问题的模型","stream":true,"messages":[{"role":"user","content":"你好"}]}' \
  | tail -30
```

- 看最后一个 `data:` 里有没有 `"usage"` 字段；若有，`completion_tokens` 是不是 0；
- 再试一次加上 `"stream_options":{"include_usage":true}`，对比结果；
- 若上面没复现，用同样的请求直接请求该渠道的上游地址（绕过 new-api），对比上游最后 chunk。

### 第 3 步：确认客户端请求体

- 让出问题的用户提供他们客户端实际发送的请求体（重点看 `stream_options`）；
- 或查看 new-api 请求日志 `other` 字段中的 `stream_options` 相关记录（如有记录）。

### 第 4 步：检查配置项

- `ForceStreamOption` 配置（constant 中）是否为 true；
- 出问题渠道的 `支持 stream_options`（SupportStreamOptions）设置。

---

## 四、预判结论与修法（确认后执行）

| 确认结果 | 修法方向 |
|---|---|
| 客户端发送了 `include_usage:false` / 空 `stream_options` | 在 `OaiStreamHandler` 中改为：**只要上游返回了 usage chunk，即使客户端没请求也透传**（或在 `include_usage` 缺失/空对象时按 true 处理），并在 `handleFinalResponse` 对 `!ShouldIncludeUsage` 也补发 usage |
| 渠道是 Claude 上游 | 在 `StreamResponseClaude2OpenAI` 的 `message_delta` 分支把 usage 填回转换后的 chunk（`claudeResponse.Usage` → `response.Usage`） |
| zero-output 重试误触发 | 检查第一次尝试是否真的有内容被丢弃（scanner 过滤逻辑），修正 `streamOutputSent` 判定或重试前先判断已写出的内容量 |

---

## 五、需要你回来反馈的信息

1. 出问题请求日志的截图/内容（token 值、渠道、模型、other 字段）；
2. 客户端实际发送的请求体（尤其 `stream_options`）；
3. curl 复现时流末尾 3-5 个 chunk 的原始内容；
4. 是否所有出问题请求都走同一个渠道/模型。
