package util

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIs1062(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "直接 1062 错误返回 true",
			err:  &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"},
			want: true,
		},
		{
			name: "wrap 后的 1062 errors.As 穿透，返回 true",
			err:  fmt.Errorf("wrap: %w", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}),
			want: true,
		},
		{
			name: "其他 MySQL 错误码（1045）返回 false",
			err:  &mysql.MySQLError{Number: 1045, Message: "Access denied"},
			want: false,
		},
		{
			name: "非 MySQL 错误返回 false",
			err:  errors.New("some other error"),
			want: false,
		},
		{
			name: "nil 返回 false",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Is1062(tt.err)
			if got != tt.want {
				t.Errorf("Is1062(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
