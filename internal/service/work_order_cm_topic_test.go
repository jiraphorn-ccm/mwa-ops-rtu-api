package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rtu-api/internal/httpx"
)

func TestCheckCmProblemTopic(t *testing.T) {
	topic := uuid.New()

	t.Run("CM requires topic", func(t *testing.T) {
		err := checkCmProblemTopics("CM", nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrCmProblemTopicRequired {
			t.Fatalf("got %#v", err)
		}
	})

	t.Run("CM accepts single topic", func(t *testing.T) {
		if err := checkCmProblemTopics("CM", &topic, nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CM accepts topic array", func(t *testing.T) {
		other := uuid.New()
		if err := checkCmProblemTopics("CM", nil, []uuid.UUID{topic, other}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CM dedupes topics in array", func(t *testing.T) {
		ids, err := normalizeProblemTopicIDs(nil, []uuid.UUID{topic, topic})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 1 {
			t.Fatalf("len=%d want 1", len(ids))
		}
	})

	t.Run("CM merges single and array", func(t *testing.T) {
		other := uuid.New()
		ids, err := normalizeProblemTopicIDs(&topic, []uuid.UUID{other})
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != 2 {
			t.Fatalf("len=%d want 2", len(ids))
		}
	})

	t.Run("PM rejects topic", func(t *testing.T) {
		err := checkCmProblemTopics("PM", &topic, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		appErr, ok := err.(*httpx.AppError)
		if !ok || appErr.Code != httpx.ErrCmProblemTopicNotAllowed {
			t.Fatalf("got %#v", err)
		}
	})
}
