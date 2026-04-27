package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepository provides admin read/update for the singleton profile row.
type AdminRepository struct {
	pool      *pgxpool.Pool
	urlPrefix string
}

// NewAdminRepository constructs an AdminRepository. urlPrefix is prepended to
// media.storage_path to produce a public avatar URL.
func NewAdminRepository(pool *pgxpool.Pool, urlPrefix string) *AdminRepository {
	return &AdminRepository{pool: pool, urlPrefix: urlPrefix}
}

// Get returns the singleton profile (id=1). Same shape as the public Get; the
// admin handler intentionally exposes Email.
func (r *AdminRepository) Get(ctx context.Context) (Profile, error) {
	const q = `
        SELECT p.id, p.full_name, p.bio, m.storage_path, p.tagline,
               p.social_links, p.location, p.email, p.updated_at
        FROM public.profile p
        LEFT JOIN public.media m ON m.id = p.avatar_id
        WHERE p.id = 1`

	var (
		out         Profile
		avatarPath  *string
		socialBytes []byte
	)
	err := r.pool.QueryRow(ctx, q).Scan(
		&out.ID, &out.FullName, &out.Bio, &avatarPath, &out.Tagline,
		&socialBytes, &out.Location, &out.Email, &out.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("admin profile get: %w", err)
	}
	if avatarPath != nil {
		url := r.avatarURL(*avatarPath)
		out.AvatarURL = &url
	}
	out.SocialLinks = map[string]string{}
	if len(socialBytes) > 0 {
		if err := json.Unmarshal(socialBytes, &out.SocialLinks); err != nil {
			return Profile{}, fmt.Errorf("admin profile decode social_links: %w", err)
		}
	}
	return out, nil
}

// Update applies a partial update to the singleton profile row. Always sets
// updated_by + updated_at.
func (r *AdminRepository) Update(ctx context.Context, in UpdateInput) (Profile, error) {
	sets := make([]string, 0, 10)
	args := make([]any, 0, 10)
	nextArg := 1
	add := func(col, cast string, v any) {
		if cast != "" {
			sets = append(sets, fmt.Sprintf("%s = $%d::%s", col, nextArg, cast))
		} else {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, nextArg))
		}
		args = append(args, v)
		nextArg++
	}

	if in.FullName != nil {
		add("full_name", "", *in.FullName)
	}
	if in.Bio != nil {
		add("bio", "", *in.Bio)
	}
	if in.AvatarID != nil {
		// non-nil pointer-to-pointer: explicit set (possibly to NULL).
		add("avatar_id", "", *in.AvatarID)
	}
	if in.Tagline != nil {
		add("tagline", "", *in.Tagline)
	}
	if in.SocialLinks != nil {
		raw, err := json.Marshal(*in.SocialLinks)
		if err != nil {
			return Profile{}, fmt.Errorf("admin profile encode social_links: %w", err)
		}
		add("social_links", "jsonb", string(raw))
	}
	if in.Location != nil {
		add("location", "", *in.Location)
	}
	if in.Email != nil {
		add("email", "", *in.Email)
	}
	add("updated_by", "", in.UpdatedBy)
	sets = append(sets, "updated_at = now()")

	stmt := fmt.Sprintf(`UPDATE public.profile SET %s WHERE id = 1`, strings.Join(sets, ", "))
	if _, err := r.pool.Exec(ctx, stmt, args...); err != nil {
		return Profile{}, fmt.Errorf("admin profile update: %w", err)
	}
	return r.Get(ctx)
}

func (r *AdminRepository) avatarURL(storagePath string) string {
	if r.urlPrefix == "" {
		return storagePath
	}
	if strings.HasSuffix(r.urlPrefix, "/") {
		return r.urlPrefix + storagePath
	}
	return r.urlPrefix + "/" + storagePath
}
