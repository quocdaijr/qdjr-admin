package posts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"  Multi   Space  ", "multi-space"},
		{"ünïcôdè!! ", "n-c-d"}, // each accented char becomes its own '-' run
		{"---leading-and-trailing---", "leading-and-trailing"},
		{"alpha--BETA__gamma", "alpha-beta-gamma"},
		{"already-good-slug", "already-good-slug"},
		{"!!!", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.want, slugify(c.in))
		})
	}
}

func TestSlugify_TruncatesTo200(t *testing.T) {
	long := strings.Repeat("abc-", 80) // 320 chars
	got := slugify(long)
	assert.LessOrEqual(t, len(got), 200)
	assert.True(t, validSlug(got), "truncated slug must remain valid")
}

func TestValidSlug(t *testing.T) {
	assert.True(t, validSlug("hello"))
	assert.True(t, validSlug("hello-world-2"))
	assert.False(t, validSlug(""))
	assert.False(t, validSlug("Hello"))      // uppercase
	assert.False(t, validSlug("hello world")) // space
	assert.False(t, validSlug("-leading"))
	assert.False(t, validSlug("trailing-"))
	assert.False(t, validSlug("double--hyphen"))
}
