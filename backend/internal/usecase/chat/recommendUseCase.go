package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"

	"github.com/otameshi/backend/internal/external/llm"
	"github.com/otameshi/backend/internal/protocol"
	"github.com/otameshi/backend/internal/service"
)

func (m *Manager) handleFunctionCall(ctx context.Context, event llm.StreamEvent) {
	switch event.FunctionName {
	case "recommend_product":
		var args llm.RecommendProductArgs
		if err := json.Unmarshal(event.FunctionArgs, &args); err != nil {
			slog.Error("failed to parse recommend_product args", "error", err)
			return
		}

		products := service.GetProducts(args.ProductIDs)
		productInfos := make([]protocol.ProductInfo, 0, len(products))
		for _, p := range products {
			productInfos = append(productInfos, protocol.ProductInfo{
				ProductID:   p.ID,
				Name:        p.Name,
				Price:       p.Price,
				Description: p.Description,
				ImageURL:    p.ImageURL,
			})
		}

		audioData, err := m.ttsService.SynthesizeWithLang(ctx, args.IntroductionSpeech, m.getDetectedLang())
		if err != nil {
			slog.Error("TTS synthesis failed", "error", err)
		}
		audioBase64 := ""
		if len(audioData) > 0 {
			audioBase64 = base64.StdEncoding.EncodeToString(audioData)
		}

		m.send(protocol.TypeProductRecommendation, protocol.ProductRecommendation{
			Transcript: args.IntroductionSpeech,
			AudioChunk: audioBase64,
			Products:   productInfos,
		})

		m.state.Transition(StateListening)
		m.silenceTimer.Resume()

		m.handlePostRecommendation(ctx)

	case "end_conversation":
		var args llm.EndConversationArgs
		if err := json.Unmarshal(event.FunctionArgs, &args); err != nil {
			slog.Error("failed to parse end_conversation args", "error", err)
			return
		}
		m.initiateClose("conversation_ended", "")
	}
}

func (m *Manager) handlePostRecommendation(ctx context.Context) {
	if m.config.PostRecommendationBehavior == "ask_interest" {
		followUp := "ご紹介した商品はいかがですか？気になるものはありましたか？"
		audioData, _ := m.ttsService.SynthesizeWithLang(ctx, followUp, m.getDetectedLang())
		audioBase64 := ""
		if len(audioData) > 0 {
			audioBase64 = base64.StdEncoding.EncodeToString(audioData)
		}
		m.send(protocol.TypeTextDone, protocol.TextDone{
			Text:       followUp,
			AudioChunk: audioBase64,
			IsFinal:    true,
		})
	}

	if m.config.RecommendationMode == "sequential" && len(m.config.SequentialItems) > 0 {
		m.seqIndex++
	}
}
