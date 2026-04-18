package errcode

import "testing"

func TestBtNodeMsgResolution_Verify(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{ErrBtNodeBareFields, "节点字段结构非法"},
		{ErrBtNodeParamMissing, "节点缺少必填参数"},
		{ErrBtNodeParamType, "节点参数类型不匹配"},
		{ErrBtNodeParamEnum, "节点参数取值不在允许集合"},
	}
	for _, c := range cases {
		if got := Msg(c.code); got != c.want {
			t.Errorf("Msg(%d) = %q, want %q", c.code, got, c.want)
		}
	}
	if ErrBtNodeBareFields != 44007 || ErrBtNodeParamMissing != 44008 ||
		ErrBtNodeParamType != 44013 || ErrBtNodeParamEnum != 44014 {
		t.Errorf("code values drifted")
	}
}
