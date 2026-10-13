// Command watch streams one course over native gRPC and prints a snapshot
// each time it changes (lessons.v1.LessonsService/WatchCourse). Over http:// it
// speaks h2c with prior knowledge; over https:// it negotiates HTTP/2 by ALPN.
//
//	QDJR_TOKEN=<access token> go run ./cmd/watch -url http://localhost:8080 -course <id>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	lessonsv1 "github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/gen/lessons/v1/lessonsv1connect"
)

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "API base URL")
	courseID := flag.String("course", "", "course id to watch")
	flag.Parse()
	token := os.Getenv("QDJR_TOKEN")
	if *courseID == "" || token == "" {
		log.Fatal("usage: QDJR_TOKEN=<token> watch -url <base> -course <id>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := watch(ctx, *baseURL, *courseID, token); err != nil {
		log.Print(err)
		stop()
		os.Exit(1)
	}
}

func watch(ctx context.Context, baseURL, courseID, token string) error {
	client := http.DefaultClient
	if strings.HasPrefix(baseURL, "http://") {
		var p http.Protocols
		p.SetUnencryptedHTTP2(true) // gRPC needs HTTP/2; no TLS locally
		client = &http.Client{Transport: &http.Transport{Protocols: &p}}
	}
	c := lessonsv1connect.NewLessonsServiceClient(connect.NewClient(connecthttp.NewTransport(client, baseURL, connecthttp.WithGRPC())))
	req := &lessonsv1.WatchCourseRequest{Id: courseID}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Authorization", "Bearer "+token)

	start := time.Now()
	stream, err := c.WatchCourse(ctx, req)
	if err != nil {
		return err
	}
	var streamErr error
	for {
		msg, err := stream.Receive()
		if err != nil {
			streamErr = err
			break
		}
		m := msg
		fmt.Printf("%s +%s  %s  %d/%d done\n", time.Now().Format(time.TimeOnly),
			time.Since(start).Round(time.Second), m.GetCourse().GetTitle(),
			m.GetCourse().GetDoneCount(), m.GetCourse().GetLessonCount())
		for _, l := range m.GetLessons() {
			status := strings.TrimPrefix(l.GetStatus().String(), "LESSON_STATUS_")
			fmt.Printf("  %2d  %-11s %s\n", l.GetPosition(), status, l.GetTitle())
		}
	}
	if errors.Is(streamErr, io.EOF) {
		streamErr = nil
	}
	if err := streamErr; err != nil && ctx.Err() == nil {
		return fmt.Errorf("stream ended after %s: %w", time.Since(start).Round(time.Second), err)
	}
	fmt.Printf("stream closed after %s\n", time.Since(start).Round(time.Second))
	return nil
}
