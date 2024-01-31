package form

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// RegisterSQLNullTypesDecodeFunc adds decoding support for sql.Null* types.
func RegisterSQLNullTypesDecodeFunc(
	d interface {
		RegisterFunc(fn DecodeFunc, types ...interface{})
	},
	nullValues ...string,
) {
	if len(nullValues) == 0 {
		nullValues = []string{"NULL"}
	}

	d.RegisterFunc(func(val string) (interface{}, error) {
		for _, null := range nullValues {
			if null == val {
				return sql.NullString{}, nil
			}
		}

		return sql.NullString{String: val, Valid: true}, nil
	}, sql.NullString{})

	d.RegisterFunc(func(val string) (interface{}, error) {
		for _, null := range nullValues {
			if null == val {
				return sql.NullInt64{}, nil
			}
		}

		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}

		return sql.NullInt64{Int64: i, Valid: true}, nil
	}, sql.NullInt64{})

	d.RegisterFunc(func(val string) (interface{}, error) {
		if len(val) < 1 {
			return nil, errors.New("no value received")
		}
		for _, null := range nullValues {
			if null == val {
				return sql.NullFloat64{}, nil
			}
		}

		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, err
		}

		return sql.NullFloat64{Float64: f, Valid: true}, nil
	}, sql.NullFloat64{})

	d.RegisterFunc(func(val string) (interface{}, error) {
		for _, null := range nullValues {
			if null == val {
				return sql.NullBool{}, nil
			}
		}

		b, err := parseBool(val)
		if err != nil {
			return nil, err
		}

		return sql.NullBool{Bool: b, Valid: true}, nil
	}, sql.NullBool{})
}

// RegisterSQLNullTypesEncodeFunc adds encoding support for sql.Null* types.
func RegisterSQLNullTypesEncodeFunc(
	e interface {
		RegisterFunc(fn EncodeFunc, types ...interface{})
	},
	nullValue string,
) {
	e.RegisterFunc(func(x interface{}) (string, error) {
		s, ok := x.(sql.NullString)
		if !ok {
			return "", fmt.Errorf("value of unexpected type received, want %T, got %T ", sql.NullString{}, x)
		}

		if !s.Valid {
			return nullValue, nil
		}

		return s.String, nil
	}, sql.NullString{})

	e.RegisterFunc(func(x interface{}) (string, error) {
		i, ok := x.(sql.NullInt64)
		if !ok {
			return "", fmt.Errorf("value of unexpected type received, want %T, got %T ", sql.NullInt64{}, x)
		}

		if !i.Valid {
			return nullValue, nil
		}

		return strconv.FormatInt(i.Int64, 10), nil
	}, sql.NullInt64{})

	e.RegisterFunc(func(x interface{}) (string, error) {
		f, ok := x.(sql.NullFloat64)
		if !ok {
			return "", fmt.Errorf("value of unexpected type received, want %T, got %T ", sql.NullFloat64{}, x)
		}

		if !f.Valid {
			return nullValue, nil
		}

		return strconv.FormatFloat(f.Float64, 'f', -1, 64), nil
	}, sql.NullFloat64{})

	e.RegisterFunc(func(x interface{}) (string, error) {
		b, ok := x.(sql.NullBool)
		if !ok {
			return "", fmt.Errorf("value of unexpected type received, want %T, got %T ", sql.NullBool{}, x)
		}

		if !b.Valid {
			return nullValue, nil
		}

		return strconv.FormatBool(b.Bool), nil
	}, sql.NullBool{})
}
