package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/samber/mo"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	captchaadapter "todoe/internal/captcha/adapter"
	captchahttp "todoe/internal/captcha/adapter/http"
	captchaapp "todoe/internal/captcha/application"
	captchadomain "todoe/internal/captcha/domain"
	"todoe/internal/event"
	"todoe/internal/messaging"
)

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

func pushToLoki(lokiURL, eventType string, payload json.RawMessage) error {
	line, _ := json.Marshal(map[string]any{
		"event_type": eventType,
		"payload":    payload,
	})
	body := lokiPush{Streams: []lokiStream{{
		Stream: map[string]string{"service": "audit", "event_type": eventType},
		Values: [][2]string{{fmt.Sprintf("%d", time.Now().UnixNano()), string(line)}},
	}}}
	data, _ := json.Marshal(body)
	resp, err := http.Post(lokiURL+"/loki/api/v1/push", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("loki push: status %d", resp.StatusCode)
	}
	return nil
}

type multiPublisher struct{ publishers []event.Publisher }

func (m *multiPublisher) Publish(ctx context.Context, e event.Event) {
	for _, p := range m.publishers {
		p.Publish(ctx, e)
	}
}

func main() {
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}
	lokiURL := os.Getenv("LOKI_URL")
	if lokiURL == "" {
		lokiURL = "http://localhost:3100"
	}
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://root:root@localhost:27017"
	}
	port := os.Getenv("CAPTCHA_PORT")
	if port == "" {
		port = "3010"
	}

	clientIO := mo.NewIOEither(func() (*mongo.Client, error) {
		return mongo.Connect(options.Client().ApplyURI(mongoURI))
	})

	conn, ch, err := messaging.Connect(amqpURL)
	if err != nil {
		log.Fatal("rabbit:", err)
	}
	defer conn.Close()

	// Declare captcha.events exchange and bind the audit queue so messages
	// buffer on disk while audit is offline — mirrors how task.events works.
	if err := messaging.DeclareTopology(ch, []messaging.Binding{
		{Exchange: messaging.CaptchaExchange, Queue: messaging.QueueAuditCaptchaEvents},
	}); err != nil {
		log.Fatal("rabbit topology:", err)
	}

	bus := event.NewEventBus()
	repo := captchaadapter.NewMongoRepository(clientIO)
	projection := captchaadapter.NewProjectionHandler(repo)
	bus.Subscribe(captchadomain.EventIssued, projection)
	bus.Subscribe(captchadomain.EventVerified, projection)

	// Publish captcha events to both the in-process bus (for projection) and
	// the captcha.events RabbitMQ exchange (consumed by cmd/audit → Loki).
	publisher := &multiPublisher{publishers: []event.Publisher{
		bus,
		messaging.NewPublisher(ch, messaging.CaptchaExchange),
	}}

	service := captchaapp.NewService(repo, publisher)
	handler := captchahttp.NewHandler(service)

	app := fiber.New()
	app.Post("/captcha", handler.Issue)
	app.Post("/captcha/:id/verify", handler.Verify)

	// Forward user onboarding events received via RabbitMQ directly to Loki.
	if err := messaging.Subscribe(ch, messaging.UserExchange, messaging.QueueOnboardingUserEvents, func(msg messaging.Message) {
		slog.Info("captcha: received event", "type", msg.Type)
		if err := pushToLoki(lokiURL, msg.Type, msg.Payload); err != nil {
			slog.Error("captcha: loki push", "err", err)
		}
	}); err != nil {
		log.Fatal("rabbit subscribe:", err)
	}

	slog.Info("captcha service listening", "port", port)
	log.Fatal(app.Listen(":" + port))
}
