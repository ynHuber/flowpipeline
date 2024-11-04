package kafkaconsumer

import (
	"bytes"
	"context"
	"log"
	"log/slog"

	"github.com/Shopify/sarama"
	"github.com/bwNetFlow/flowpipeline/pb"
	"google.golang.org/protobuf/encoding/protodelim"
)

// Handler represents a Sarama consumer group consumer
type Handler struct {
	ready  chan bool
	flows  chan *pb.EnrichedFlow
	cancel context.CancelFunc
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *Handler) Setup(session sarama.ConsumerGroupSession) error {
	log.Println("[info] KafkaConsumer: Received new partition set to claim:", session.Claims()) // TODO: print those
	// reopen flows channel
	h.flows = make(chan *pb.EnrichedFlow)
	// Mark the consumer as ready
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *Handler) Cleanup(sarama.ConsumerGroupSession) error {
	close(h.flows)
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().

func (h *Handler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			var msg pb.ProtoProducerMessage
			if err := protodelim.UnmarshalFrom(bytes.NewReader(message.Value), &msg); err != nil {
				slog.Error("error unmarshalling message", slog.String("error", err.Error()))
				continue
			}
			h.flows <- &msg.EnrichedFlow
		case <-session.Context().Done():
			return nil
		}
	}
}
