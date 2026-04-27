package contact

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool, ctx
}

func TestRepository_Create_WithIP(t *testing.T) {
	pool, ctx := newPool(t)
	repo := NewRepository(pool)

	subject := "hello-" + uuid.New().String()[:8]
	in := CreateInput{
		Name:      "Alice",
		Email:     "alice@example.com",
		Subject:   &subject,
		Body:      "hi there",
		IP:        "203.0.113.5",
		UserAgent: "go-test/1.0",
	}
	out, err := repo.Create(ctx, in)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, out.ID)
	assert.False(t, out.CreatedAt.IsZero())

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from public.contact_messages where id = $1`, out.ID)
	})

	var (
		gotName, gotEmail, gotBody, gotUA string
		gotSubject, gotIP                 *string
	)
	err = pool.QueryRow(ctx,
		`select name, email, subject, body, host(ip)::text, user_agent
		   from public.contact_messages where id = $1`, out.ID,
	).Scan(&gotName, &gotEmail, &gotSubject, &gotBody, &gotIP, &gotUA)
	require.NoError(t, err)
	assert.Equal(t, "Alice", gotName)
	assert.Equal(t, "alice@example.com", gotEmail)
	require.NotNil(t, gotSubject)
	assert.Equal(t, subject, *gotSubject)
	assert.Equal(t, "hi there", gotBody)
	require.NotNil(t, gotIP)
	assert.Equal(t, "203.0.113.5", *gotIP)
	assert.Equal(t, "go-test/1.0", gotUA)
}

func TestRepository_Create_NoIPNoSubject(t *testing.T) {
	pool, ctx := newPool(t)
	repo := NewRepository(pool)

	in := CreateInput{
		Name:  "Bob",
		Email: "bob@example.com",
		Body:  "no metadata",
	}
	out, err := repo.Create(ctx, in)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `delete from public.contact_messages where id = $1`, out.ID)
	})

	var (
		ipNull, subjectNull, uaNull bool
	)
	err = pool.QueryRow(ctx,
		`select ip is null, subject is null, user_agent is null
		   from public.contact_messages where id = $1`, out.ID,
	).Scan(&ipNull, &subjectNull, &uaNull)
	require.NoError(t, err)
	assert.True(t, ipNull, "empty IP should persist as NULL")
	assert.True(t, subjectNull, "nil subject should persist as NULL")
	assert.True(t, uaNull, "empty user_agent should persist as NULL")
}
