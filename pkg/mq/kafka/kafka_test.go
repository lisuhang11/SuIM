package kafka

import (
	"context"
	"testing"

	"github.com/IBM/sarama"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
)

func TestBuildProducerConfigAcksAndCompression(t *testing.T) {
	cfg, err := BuildProducerConfig(Config{ProducerAck: "wait_for_local", CompressType: "snappy"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Producer.RequiredAcks != sarama.WaitForLocal {
		t.Fatalf("acks=%v", cfg.Producer.RequiredAcks)
	}
	if cfg.Producer.Compression != sarama.CompressionSnappy {
		t.Fatalf("compression=%v", cfg.Producer.Compression)
	}
	if cfg.Producer.Partitioner == nil {
		t.Fatal("expected hash partitioner")
	}
}

func TestBuildConsumerGroupConfig(t *testing.T) {
	cfg, err := BuildConsumerGroupConfig(&Config{}, sarama.OffsetNewest, true)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Consumer.Offsets.AutoCommit.Enable {
		t.Fatal("expected auto commit")
	}
	if cfg.Consumer.Offsets.Initial != sarama.OffsetNewest {
		t.Fatalf("initial=%d", cfg.Consumer.Offsets.Initial)
	}
}

func TestHeadersRoundTrip(t *testing.T) {
	ctx := WithOperationID(rpcctx.WithUser(context.Background(), "u1"), "op-9")
	headers := HeadersFromContext(ctx)
	if len(headers) != 2 {
		t.Fatalf("headers=%d", len(headers))
	}
	ptrs := make([]*sarama.RecordHeader, 0, len(headers))
	for i := range headers {
		h := headers[i]
		ptrs = append(ptrs, &h)
	}
	got := ContextFromHeaders(context.Background(), ptrs)
	if OperationID(got) != "op-9" {
		t.Fatalf("operationID=%q", OperationID(got))
	}
	if rpcctx.UserID(got) != "u1" {
		t.Fatalf("user=%q", rpcctx.UserID(got))
	}
}

func TestNewProducerValidation(t *testing.T) {
	if _, err := NewProducer(nil, "t"); err == nil {
		t.Fatal("expected nil config error")
	}
	if _, err := NewProducer(&Config{Addr: []string{"127.0.0.1:9092"}}, ""); err == nil {
		t.Fatal("expected empty topic error")
	}
	if _, err := NewProducer(&Config{}, "t"); err == nil {
		t.Fatal("expected empty addr error")
	}
}

func TestProducerSendEmpty(t *testing.T) {
	p := &Producer{topic: "t", producer: stubSyncProducer{}}
	if err := p.Send(context.Background(), "k", nil); err == nil {
		t.Fatal("expected empty value error")
	}
}

type stubSyncProducer struct{}

func (stubSyncProducer) SendMessage(*sarama.ProducerMessage) (int32, int64, error) {
	return 0, 0, nil
}
func (stubSyncProducer) SendMessages([]*sarama.ProducerMessage) error { return nil }
func (stubSyncProducer) Close() error                                 { return nil }
func (stubSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag      { return 0 }
func (stubSyncProducer) IsTransactional() bool                        { return false }
func (stubSyncProducer) BeginTxn() error                              { return nil }
func (stubSyncProducer) CommitTxn() error                             { return nil }
func (stubSyncProducer) AbortTxn() error                              { return nil }
func (stubSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (stubSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
