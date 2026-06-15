package piagent

import (
	"gorm.io/gorm"
)

type gormDBWrapper struct {
	db *gorm.DB
}

func (w *gormDBWrapper) Model(value any) DB {
	return &gormDBWrapper{db: w.db.Model(value)}
}

func (w *gormDBWrapper) Where(query any, args ...any) DB {
	return &gormDBWrapper{db: w.db.Where(query, args...)}
}

func (w *gormDBWrapper) Order(value any) DB {
	return &gormDBWrapper{db: w.db.Order(value)}
}

func (w *gormDBWrapper) Count(count *int64) DB {
	return &gormDBWrapper{db: w.db.Count(count)}
}

func (w *gormDBWrapper) Find(dest any, conds ...any) DB {
	return &gormDBWrapper{db: w.db.Find(dest, conds...)}
}

func (w *gormDBWrapper) First(dest any, conds ...any) DB {
	return &gormDBWrapper{db: w.db.First(dest, conds...)}
}

func (w *gormDBWrapper) Offset(offset int) DB {
	return &gormDBWrapper{db: w.db.Offset(offset)}
}

func (w *gormDBWrapper) Limit(limit int) DB {
	return &gormDBWrapper{db: w.db.Limit(limit)}
}

func (w *gormDBWrapper) Create(value any) DB {
	return &gormDBWrapper{db: w.db.Create(value)}
}

func (w *gormDBWrapper) Error() error {
	return w.db.Error
}
