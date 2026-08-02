package cache

import (
	"context"

	"SuIM/suim-sdk-core/pkg/sdkerrs"
)

// UserCache mirrors openim-sdk-core UserCache: memory → local DB → remote query.
func NewUserCache[K comparable, V any](
	getKeyFunc func(value V) K,
	singleDBFunc func(ctx context.Context, key K) (V, error),
	queryFunc func(ctx context.Context, keys []K) ([]V, error),
) *UserCache[K, V] {
	return &UserCache[K, V]{
		Cache:        NewCache[K, V](),
		getKeyFunc:   getKeyFunc,
		singleDBFunc: singleDBFunc,
		queryFunc:    queryFunc,
	}
}

type UserCache[K comparable, V any] struct {
	*Cache[K, V]
	getKeyFunc   func(value V) K
	singleDBFunc func(ctx context.Context, key K) (V, error)
	queryFunc    func(ctx context.Context, keys []K) ([]V, error)
}

func (m *UserCache[K, V]) Fetch(ctx context.Context, key K) (V, error) {
	var nilData V
	if data, ok := m.Load(key); ok {
		return data, nil
	}
	fetchedData, err := m.fetch(ctx, key)
	if err != nil {
		return nilData, err
	}
	m.Store(key, fetchedData)
	return fetchedData, nil
}

func (m *UserCache[K, V]) BatchFetch(ctx context.Context, keys []K) (map[K]V, error) {
	res := make(map[K]V)
	var missing []K
	for _, key := range keys {
		if data, ok := m.Load(key); ok {
			res[key] = data
		} else {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return res, nil
	}
	writeData, err := m.batchFetch(ctx, missing)
	if err != nil {
		return nil, err
	}
	for _, data := range writeData {
		k := m.getKeyFunc(data)
		res[k] = data
		m.Store(k, data)
	}
	return res, nil
}

func (m *UserCache[K, V]) batchFetch(ctx context.Context, keys []K) ([]V, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	if m.queryFunc == nil {
		return nil, nil
	}
	queryData, err := m.queryFunc(ctx, keys)
	if err != nil {
		return nil, err
	}
	if len(queryData) == 0 {
		return nil, sdkerrs.ErrUserNotFound
	}
	return queryData, nil
}

func (m *UserCache[K, V]) fetch(ctx context.Context, key K) (V, error) {
	var writeData V
	if m.singleDBFunc != nil {
		dbData, err := m.singleDBFunc(ctx, key)
		if err == nil {
			return dbData, nil
		}
	}
	if m.queryFunc != nil {
		queryData, err := m.queryFunc(ctx, []K{key})
		if err != nil {
			return writeData, err
		}
		if len(queryData) > 0 {
			return queryData[0], nil
		}
		return writeData, sdkerrs.ErrUserNotFound
	}
	return writeData, nil
}
