package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
)

func TestCheckCmProblemTopic(t *testing.T) {
	topic := uuid.New()

	t.Run("CM requires topic", func(t *testing.T) {
		err := checkCmProblemTopic("CM", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrCmProblemTopicRequired {
			t.Fatalf("got %#v", err)
		}
	})

	t.Run("CM rejects nil UUID", func(t *testing.T) {
		nilTopic := uuid.Nil
		err := checkCmProblemTopic("CM", &nilTopic)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("CM accepts topic", func(t *testing.T) {
		if err := checkCmProblemTopic("CM", &topic); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("PM rejects topic", func(t *testing.T) {
		err := checkCmProblemTopic("PM", &topic)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrCmProblemTopicNotAllowed {
			t.Fatalf("got %#v", err)
		}
	})
}
