package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefixBillingLogContent(t *testing.T) {
	assert.Equal(t, "模型倍率 1.00", PrefixBillingLogContent(BillingSourceWallet, "模型倍率 1.00"))
	assert.Equal(t, "[GPT扣费] 模型倍率 1.00", PrefixBillingLogContent(BillingSourceGptWallet, "模型倍率 1.00"))
	assert.Equal(t, "[GPT扣费]", PrefixBillingLogContent(BillingSourceGptWallet, ""))
}

func TestGenerateMjOtherInfo_AppendsBillingSource(t *testing.T) {
	other := GenerateMjOtherInfo(&relaycommon.RelayInfo{
		BillingSource: BillingSourceGptWallet,
	}, types.PriceData{
		ModelPrice: 0.02,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 1.5,
		},
	})

	assert.Equal(t, BillingSourceGptWallet, other["billing_source"])
}

func TestGenerateTextOtherInfo_AppendsGptBillingBreakdown(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	now := time.Now()

	other := GenerateTextOtherInfo(ctx, &relaycommon.RelayInfo{
		BillingSource:           BillingSourceGptWallet,
		InitialPreConsumedQuota: 2000,
		BillingPostDeltaQuota:   700,
		FirstResponseTime:       now,
		StartTime:               now,
		ChannelMeta:             &relaycommon.ChannelMeta{},
	}, 1, 1, 1, 0, 0, 0, -1)

	assert.Equal(t, BillingSourceGptWallet, other["billing_source"])
	assert.Equal(t, 2000, other["gpt_pre_consumed"])
	assert.Equal(t, 700, other["gpt_post_delta"])
}

func TestTaskBillingOther_IncludesBillingSource(t *testing.T) {
	other := taskBillingOther(&model.Task{
		PrivateData: model.TaskPrivateData{
			BillingSource:  BillingSourceSubscription,
			SubscriptionId: 123,
		},
	})

	assert.Equal(t, BillingSourceSubscription, other["billing_source"])
	assert.Equal(t, 123, other["subscription_id"])
}

func TestLogTaskConsumption_GptWalletWritesBillingSourceAndPrefix(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)

	const userID, tokenID, channelID = 41, 41, 41
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "sk-gpt-task", 10000)
	seedChannel(t, channelID)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/tasks/submit", nil)
	ctx.Set("token_name", "test_token")
	ctx.Set("username", "test_user")

	info := &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		UsingGroup:      "gpt-only",
		OriginModelName: "task-model",
		BillingSource:   BillingSourceGptWallet,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: channelID,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			Action: "video_submit",
		},
		PriceData: types.PriceData{
			Quota:      123,
			ModelPrice: 0.02,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}

	LogTaskConsumption(ctx, info)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.Contains(t, log.Content, "[GPT扣费]")

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, BillingSourceGptWallet, other["billing_source"])
}
