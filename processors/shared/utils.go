package processor_shared

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func Int4(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func Int8[T ~int | ~int32 | ~int64](v T) pgtype.Int8 {
	return pgtype.Int8{Int64: int64(v), Valid: true}
}

func Float8[T ~int | ~int32 | ~int64 | ~float32 | ~float64](v T) pgtype.Float8 {
	return pgtype.Float8{
		Float64: float64(v),
		Valid:   true,
	}
}

func UUID(v string) (*pgtype.UUID, error) {
	parsedUUID, err := uuid.Parse(v)
	if err != nil {
		return nil, err
	}

	return &pgtype.UUID{
		Bytes: parsedUUID,
		Valid: true,
	}, nil
}
