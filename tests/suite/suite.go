package suite

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"sso/internal/app"
	"sso/internal/config"
	"sso/internal/lib/logger/handlers/slogdiscard"

	ssov1 "github.com/bolatl/protos/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Suite struct {
	*testing.T                  // Потребуется для вызова методов *testing.T внутри Suite
	Cfg        *config.Config   // Конфигурация приложения
	AuthClient ssov1.AuthClient // Клиент для взаимодействия с gRPC-сервером
	app        *app.App         // Приложение для управления gRPC сервером
}

const (
	grpcHost = "localhost"
)

// New creates new test suite.
func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()
	t.Parallel()

	cfg := config.MustLoadPath(configPath())

	// Get a free port for this test instance
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Initialize logger for tests
	log := slogdiscard.NewDiscardLogger()

	// Initialize and start gRPC server on the free port
	application := app.New(log, port, cfg.StoragePath, cfg.TokenTtl)
	go func() {
		if err := application.GRPCSrv.Run(); err != nil {
			t.Errorf("gRPC server failed: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	ctx, cancelCtx := context.WithTimeout(context.Background(), cfg.GRPC.Timeout)

	t.Cleanup(func() {
		t.Helper()
		cancelCtx()
		application.GRPCSrv.Stop()
	})

	// Connect to the server using the dynamically allocated port
	address := net.JoinHostPort(grpcHost, strconv.Itoa(port))
	cc, err := grpc.DialContext(context.Background(),
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials())) // Используем insecure-коннект для тестов
	if err != nil {
		t.Fatalf("grpc server connection failed: %v", err)
	}

	t.Cleanup(func() {
		cc.Close()
	})

	return ctx, &Suite{
		T:          t,
		Cfg:        cfg,
		AuthClient: ssov1.NewAuthClient(cc),
		app:        application,
	}
}

func configPath() string {
	const key = "CONFIG_PATH"

	if v := os.Getenv(key); v != "" {
		return v
	}

	return "../config/local.yaml"
}

