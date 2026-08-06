package openai

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/otameshi/backend/internal/config"
)

// AssessInterestArgs is the function call payload for assess_interest.
type AssessInterestArgs struct {
	InterestLevel       int      `json:"interest_level"`
	DetectedPreferences []string `json:"detected_preferences"`
	ResponseText        string   `json:"response_text"`
	TriggerUtterance    string   `json:"trigger_utterance"`
}

// RecommendProductArgs is the function call payload for recommend_product.
type RecommendProductArgs struct {
	ProductID          string `json:"product_id"`
	IntroductionSpeech string `json:"introduction_speech"`
	Reason             string `json:"reason"`
}

// EndConversationArgs is the function call payload for end_conversation.
type EndConversationArgs struct {
	ClosingSpeech string `json:"closing_speech"`
}

// StubClient implements RealtimeClient without calling the real OpenAI API.
type StubClient struct{}

// NewStubClient creates a new StubClient.
func NewStubClient() *StubClient {
	return &StubClient{}
}

var genericResponses = []string{
	"はい、何かお探しですか？ハイチュウのラインナップをご紹介できますよ！",
	"こんにちは！今日はどんなお菓子をお探しですか？",
	"ハイチュウはフルーツの味が楽しめるソフトキャンディです。気になるフレーバーはありますか？",
	"お気軽にお声がけください。おすすめの商品をご紹介しますよ！",
	"もちもちした食感が特徴のハイチュウ、一度試してみませんか？",
}

var responseIndex int

func (s *StubClient) SendUserMessage(ctx context.Context, text string) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 32)

	slog.Info("LLM request (stub)",
		"mode", "stub",
		"user_message", text,
		"params", map[string]interface{}{
			"mode":         "stub",
			"user_message": text,
		},
	)

	go func() {
		defer close(ch)

		lower := strings.ToLower(text)

		switch {
		case containsAny(lower, "ありがとう", "バイバイ", "さようなら", "終わり", "もういい"):
			s.sendEndConversation(ctx, ch)

		case containsAny(lower, "グレープ", "ぶどう"):
			s.streamTextAndDone(ctx, ch,
				"グレープ味のハイチュウをおすすめします！ジューシーな果汁感がたまらない定番フレーバーです。")

		case containsAny(lower, "ストロベリー", "いちご"):
			s.streamTextAndDone(ctx, ch,
				"ストロベリー味のハイチュウをおすすめします！いちごの華やかな香りが楽しめますよ。")

		case containsAny(lower, "グリーンアップル", "りんご", "青りんご"):
			s.streamTextAndDone(ctx, ch,
				"グリーンアップル味のハイチュウをおすすめします！爽やかな酸味がクセになりますよ。")

		case containsAny(lower, "全部", "種類", "ラインナップ"):
			s.streamTextAndDone(ctx, ch,
				"ハイチュウの全ラインナップをご紹介します！グレープ、ストロベリー、グリーンアップルの3種類をご用意しております。")

		case containsAny(lower, "おすすめ", "商品", "何がある", "どんな"):
			s.streamTextAndDone(ctx, ch,
				"おすすめのハイチュウをご紹介しますね！人気の3フレーバーをご覧ください。")

		default:
			s.sendGenericResponse(ctx, ch)
		}
	}()

	return ch, nil
}

func (s *StubClient) RequestGreeting(ctx context.Context) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 32)
	slog.Info("LLM request (stub)", "mode", "stub", "kind", "greeting")
	go func() {
		defer close(ch)
		s.streamTextAndDone(ctx, ch, "こんにちは！おためしちゃんです。今日はどんなお菓子をお探しですか？")
	}()
	return ch, nil
}

func (s *StubClient) AppendAssistantMessage(text string) {}

func (s *StubClient) RestoreHistory(_ []ChatMessage) {}
func (s *StubClient) History() []ChatMessage { return nil }

func (s *StubClient) UpdateSystemPrompt(prompt string) {}

func (s *StubClient) InjectProductCatalog(catalogText string) {}

func (s *StubClient) SetRecommendationTools(products []config.Product) {}

func (s *StubClient) RequestRecommendation(ctx context.Context) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 32)
	slog.Info("LLM request (stub)", "mode", "stub", "kind", "recommendation")
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		args := RecommendProductArgs{
			ProductID:          "h001",
			IntroductionSpeech: "こちらの商品をおすすめします！",
			Reason:             "人気のフレーバーです",
		}
		raw, _ := json.Marshal(args)
		select {
		case <-ctx.Done():
			return
		case ch <- StreamEvent{
			Type:         "function_call",
			FunctionName: "recommend_product",
			FunctionArgs: raw,
		}:
		}
	}()
	return ch, nil
}

func (s *StubClient) RequestSequentialRecommendation(ctx context.Context, productID string) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 32)
	slog.Info("LLM request (stub)", "mode", "stub", "kind", "sequential_recommendation", "product_id", productID)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		args := RecommendProductArgs{
			ProductID:          productID,
			IntroductionSpeech: "こちらの商品をおすすめします！",
			Reason:             "人気のフレーバーです",
		}
		raw, _ := json.Marshal(args)
		select {
		case <-ctx.Done():
			return
		case ch <- StreamEvent{
			Type:         "function_call",
			FunctionName: "recommend_product",
			FunctionArgs: raw,
		}:
		}
	}()
	return ch, nil
}

func (s *StubClient) Close() error {
	return nil
}

func (s *StubClient) streamTextAndDone(ctx context.Context, ch chan<- StreamEvent, speech string) {
	s.streamText(ctx, ch, speech)
	select {
	case <-ctx.Done():
		return
	case ch <- StreamEvent{Type: "text_done", Text: speech}:
	}
}

func (s *StubClient) sendEndConversation(ctx context.Context, ch chan<- StreamEvent) {
	speech := "ご利用ありがとうございました！またお気軽にお声がけくださいね。"

	select {
	case <-ctx.Done():
		return
	case <-time.After(300 * time.Millisecond):
	}

	args := EndConversationArgs{
		ClosingSpeech: speech,
	}
	raw, _ := json.Marshal(args)
	select {
	case <-ctx.Done():
		return
	case ch <- StreamEvent{
		Type:         "function_call",
		FunctionName: "end_conversation",
		FunctionArgs: raw,
	}:
	}
}

func (s *StubClient) sendGenericResponse(ctx context.Context, ch chan<- StreamEvent) {
	resp := genericResponses[responseIndex%len(genericResponses)]
	responseIndex++
	s.streamTextAndDone(ctx, ch, resp)
}

func (s *StubClient) streamText(ctx context.Context, ch chan<- StreamEvent, text string) {
	runes := []rune(text)
	chunkSize := 4
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])

		select {
		case <-ctx.Done():
			return
		case ch <- StreamEvent{Type: "text_delta", Text: chunk}:
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Millisecond):
		}
	}
}

func containsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
