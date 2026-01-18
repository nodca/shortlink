package repo

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"day.local/internal/app/shortlink"
	"day.local/internal/app/shortlink/cache"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrShortlinkNotFound = errors.New("shortlink not found")
var ErrAlreadyDisabled = errors.New("shortlink already disabled")
var ErrShortlinkCodeAlreadyExists = errors.New("shortlink code already exists")
var ErrShortlinkURLAlreadyHasDifferentCode = errors.New("shortlink url already has different code")

type ShortlinksMetaData struct {
	URL       string    `json:"url"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserShortlink struct {
	Code       string    `json:"code"`
	URL        string    `json:"url"`
	Disabled   bool      `json:"disabled"`
	CreatedAt  time.Time `json:"created_at"`
	ClickCount int64     `json:"click_count"`
}

type ShortlinksRepo struct {
	db    *pgxpool.Pool
	cache *cache.ShortlinkCache
}

func NewShortlinksRepo(db *pgxpool.Pool, cache *cache.ShortlinkCache) *ShortlinksRepo {
	return &ShortlinksRepo{
		db:    db,
		cache: cache,
	}
}

/*
将用户的长连接，生成短码并保存到数据库
传入http请求的上下文c.Req.Context()
*/
func (s *ShortlinksRepo) Create(ctx context.Context, url string, createdBy *int64) (string, error) {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	//开启事务
	tx, err := s.db.Begin(dbctx)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	defer tx.Rollback(dbctx) //事务提交成功后 rollback 会无效/返回错误，可忽略

	//插入 url并获取id
	var id int64
	var code string

	if err := tx.
		QueryRow(dbctx, "INSERT INTO shortlinks (url,disabled) VALUES ($1,$2) ON CONFLICT (url) DO UPDATE SET url=EXCLUDED.url RETURNING id, COALESCE(code,'')", url, false).
		Scan(&id, &code); err != nil {
		slog.Error(err.Error())
		return "", err
	}

	if code == "" {
		newCode, err := shortlink.SqidsEncode(uint64(id))
		if err != nil {
			slog.Error(err.Error())
			return "", err
		}

		// Only set code when missing; if another transaction already set it, fall back to SELECT.
		if err := tx.
			QueryRow(dbctx, "UPDATE shortlinks SET code=$1 WHERE id=$2 AND (code IS NULL OR code='') RETURNING code", newCode, id).
			Scan(&code); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				if err := tx.QueryRow(dbctx, "SELECT code FROM shortlinks WHERE id=$1", id).Scan(&code); err != nil {
					slog.Error(err.Error())
					return "", err
				}
			} else {
				slog.Error(err.Error())
				return "", err
			}
		}
	}

	if createdBy != nil {
		_, err := tx.Exec(dbctx, "INSERT INTO user_shortlinks (user_id,shortlink_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", *createdBy, id)
		if err != nil {
			slog.Error(err.Error())
			return "", err
		}
	}

	if err := tx.Commit(dbctx); err != nil {
		slog.Error(err.Error())
		return "", err
	}

	// 写缓存/覆盖负缓存：创建成功后立刻写入，避免此前命中 "__nil__" 导致短码暂时不可用。
	if s.cache != nil && code != "" {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		_ = s.cache.Set(cacheCtx, code, url)
	}

	return code, nil
}

// CreateWithCustomCode 创建短链，并尝试使用用户自定义 code。
//
// 行为约定：
// - code 已被占用：返回 ErrShortlinkCodeAlreadyExists
// - url 已存在且 code 不同：返回 ErrShortlinkURLAlreadyHasDifferentCode
// - url 已存在且 code 为空：会尝试把 code 更新为自定义 code
// - url 已存在且 code 相同：幂等返回该 code
func (s *ShortlinksRepo) CreateWithCustomCode(ctx context.Context, url string, code string, createdBy *int64) (string, error) {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := s.db.Begin(dbctx)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	defer tx.Rollback(dbctx)

	// 1) 尝试直接插入（url/codel 都有唯一约束）
	var id int64
	var gotCode string
	err = tx.QueryRow(dbctx,
		"INSERT INTO shortlinks (url, code, disabled) VALUES ($1, $2, false) ON CONFLICT (url) DO NOTHING RETURNING id, code",
		url, code,
	).Scan(&id, &gotCode)
	if err == nil {
		// inserted new row with custom code
	} else if errors.Is(err, pgx.ErrNoRows) {
		// url 已存在，查出当前 code
		if err := tx.QueryRow(dbctx, "SELECT id, COALESCE(code,'') FROM shortlinks WHERE url=$1", url).Scan(&id, &gotCode); err != nil {
			slog.Error(err.Error())
			return "", err
		}
		if gotCode != "" && gotCode != code {
			return "", ErrShortlinkURLAlreadyHasDifferentCode
		}
		if gotCode == "" {
			// 尝试填充缺失 code（可能会与其它短码冲突）
			if err := tx.QueryRow(dbctx,
				"UPDATE shortlinks SET code=$1 WHERE url=$2 AND (code IS NULL OR code='') RETURNING code",
				code, url,
			).Scan(&gotCode); err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					return "", ErrShortlinkCodeAlreadyExists
				}
				slog.Error(err.Error())
				return "", err
			}
		}
	} else {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// unique violation: code 冲突
			if strings.Contains(strings.ToLower(pgErr.ConstraintName), "code") {
				return "", ErrShortlinkCodeAlreadyExists
			}
		}
		slog.Error(err.Error())
		return "", err
	}

	if createdBy != nil {
		_, err := tx.Exec(dbctx, "INSERT INTO user_shortlinks (user_id,shortlink_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", *createdBy, id)
		if err != nil {
			slog.Error(err.Error())
			return "", err
		}
	}

	if err := tx.Commit(dbctx); err != nil {
		slog.Error(err.Error())
		return "", err
	}

	// 写缓存/覆盖负缓存：自定义短码创建成功后立刻写入。
	if s.cache != nil && gotCode != "" {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		_ = s.cache.Set(cacheCtx, gotCode, url)
	}

	return gotCode, nil
}

// 用户访问短码 code,返回对应的长链接url
func (s *ShortlinksRepo) Resolve(ctx context.Context, code string) string {
	//先查缓存
	if s.cache != nil {
		if url, _ := s.cache.Get(ctx, code); url != "" {
			if url == "__nil__" {
				return "" //命中负缓存
			}
			return url
		}
	}

	//查数据库
	dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	rows := s.db.QueryRow(dbctx, "SELECT url FROM shortlinks WHERE code=$1 AND disabled=false", code)
	var url string
	if err := rows.Scan(&url); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if s.cache != nil {
				s.cache.SetNotFound(ctx, code)
			}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error(err.Error())
		}
		return ""
	}

	//写缓存
	if s.cache != nil && url != "" {
		s.cache.Set(ctx, code, url)
	}
	return url
}

func (s *ShortlinksRepo) FindByCode(ctx context.Context, code string) (*ShortlinksMetaData, error) {
	var data ShortlinksMetaData
	dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if err := s.db.
		QueryRow(dbctx, "SELECT url,disabled,created_at,updated_at FROM shortlinks WHERE code=$1", code).
		Scan(&data.URL, &data.Disabled, &data.CreatedAt, &data.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShortlinkNotFound
		}
		slog.Error(err.Error())
		return nil, err
	}
	return &data, nil
}

func (s *ShortlinksRepo) DisableByCode(ctx context.Context, code string) error {
	dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// Try to disable when currently enabled.
	var ok int
	err := s.db.QueryRow(dbctx, "UPDATE shortlinks SET disabled=true, updated_at=now() WHERE code=$1 AND disabled=false RETURNING 1", code).Scan(&ok)
	if err == nil {
		if s.cache != nil {
			s.cache.Delete(ctx, code)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error(err.Error())
		return err
	}

	// No rows updated: either not found, or already disabled.
	var disabled bool
	if err := s.db.QueryRow(dbctx, "SELECT disabled FROM shortlinks WHERE code=$1", code).Scan(&disabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrShortlinkNotFound
		}
		slog.Error(err.Error())
		return err
	}
	if disabled {
		return ErrAlreadyDisabled
	}

	// Should not happen; the row exists and is enabled, but UPDATE matched no rows.
	return errors.New("shortlink disable failed")
}

func (u *ShortlinksRepo) ListByUserID(ctx context.Context, userID int64, limit int) ([]UserShortlink, error) {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := u.db.Query(dbctx, "SELECT s.code,s.url,s.disabled,s.click_count,us.created_at FROM user_shortlinks us JOIN shortlinks s ON s.id=us.shortlink_id WHERE us.user_id=$1 ORDER BY us.created_at DESC LIMIT $2", userID, limit)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	defer rows.Close()

	var result []UserShortlink
	for rows.Next() {
		var item UserShortlink
		if err := rows.Scan(&item.Code, &item.URL, &item.Disabled, &item.ClickCount, &item.CreatedAt); err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	return result, nil
}

func (u *ShortlinksRepo) RemoveFromUserList(ctx context.Context, userID int64, code string) error {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := u.db.Exec(dbctx, `
          DELETE FROM user_shortlinks us
          USING shortlinks s
          WHERE us.user_id = $1
            AND us.shortlink_id = s.id
            AND s.code = $2
      `, userID, code)

	if err != nil {
		slog.Error(err.Error())
		return err
	}
	return nil
}

type ClickStats struct {
	ID        int64     `json:"id"` //用于下一次查询的分页cursor
	ClickedAt time.Time `json:"clicked_at"`
	Referer   string    `json:"referer"`
	UserAgent string    `json:"user_agent"`
}

type StatsResponse struct {
	TotalClicks  uint64       `json:"total_clicks"`
	RecentClicks []ClickStats `json:"recent_clicks"`
	NextCursor   *int64       `json:"next_cursor,omitempty"`
}

func (u *ShortlinksRepo) ListStatsByCode(ctx context.Context, code string, limit int, cursor int64) (*StatsResponse, error) {
	dbctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	//查总点击数
	var TotalClicks uint64
	if err := u.db.QueryRow(dbctx, `SELECT click_count FROM shortlinks WHERE code = $1`, code).Scan(&TotalClicks); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShortlinkNotFound
		}
		slog.Error(err.Error())
		return nil, err
	}
	//查明细列表
	var rows pgx.Rows
	var err error
	if cursor == 0 {
		rows, err = u.db.Query(dbctx, `SELECT id,clicked_at,referer,user_agent FROM click_stats WHERE code = $1 ORDER BY id DESC LIMIT $2`, code, limit)
	} else {
		rows, err = u.db.Query(dbctx, `SELECT id,clicked_at,referer,user_agent FROM click_stats WHERE code = $1 AND id<$2 ORDER BY id DESC LIMIT $3`, code, cursor, limit)
	}
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	defer rows.Close()

	var clicks []ClickStats
	for rows.Next() {
		var item ClickStats
		if err := rows.Scan(&item.ID, &item.ClickedAt, &item.Referer, &item.UserAgent); err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		clicks = append(clicks, item)
	}
	if err := rows.Err(); err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	var NextCursor *int64
	if len(clicks) == limit {
		//还有下一页
		NextCursor = &clicks[len(clicks)-1].ID
	}

	return &StatsResponse{
		TotalClicks:  TotalClicks,
		RecentClicks: clicks,
		NextCursor:   NextCursor,
	}, nil

}

func (u *ShortlinksRepo) UserOwnsShortlink(ctx context.Context, userID int64, code string) (bool, error) {
	dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var exists bool
	err := u.db.QueryRow(dbctx, `SELECT EXISTS(
	SELECT 1 FROM user_shortlinks us JOIN shortlinks s ON s.id=us.shortlink_id WHERE us.user_id = $1 AND s.code = $2)`, userID, code).Scan(&exists)
	if err != nil {
		slog.Error(err.Error())
		return false, err
	}
	return exists, nil
}
