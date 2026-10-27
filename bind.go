package flare

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
)

// returned by Bind for content-types other than json/form
var ErrUnsupportedMediaType = errors.New("flare: unsupported media type")

// decode body from content-type (json or form-urlencoded)
// call from handlers only, not from router hot path
func (c *Ctx) Bind(v any) error {
	switch mediaType(c.GetHeader("Content-Type")) {
	case "application/json":
		return c.BindJSON(v)
	case "application/x-www-form-urlencoded":
		return c.BindForm(v)
	case "":
		body := c.PostBody()
		if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
			return c.BindJSON(v)
		}
		return c.BindForm(v)
	default:
		return ErrUnsupportedMediaType
	}
}

// fill exported string/int fields from post args (form tag, else json, else name)
// reflect-based, also keep off hot path
func (c *Ctx) BindForm(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return errors.New("flare: BindForm target must be a non-nil struct pointer")
	}
	rv = rv.Elem()
	rt := rv.Type()
	args := c.PostArgs()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		key := formKey(f)
		if key == "-" || key == "" {
			continue
		}
		raw := args.Peek(key)
		if raw == nil {
			continue
		}
		s := b2s(raw)
		fv := rv.Field(i)
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(s)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(n)
		}
	}
	return nil
}

// form tag, else json, else field name
func formKey(f reflect.StructField) string {
	if tag := f.Tag.Get("form"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	if tag := f.Tag.Get("json"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return f.Name
}

// strip params/; from content-type
func mediaType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// true when text/html appears before application/json in Accept
func prefersHTML(accept string) bool {
	if accept == "" {
		return false
	}
	// first-match only
	a := strings.ToLower(accept)
	hi, hj := strings.Index(a, "text/html"), strings.Index(a, "application/json")
	if hi < 0 {
		return false
	}
	if hj < 0 {
		return true
	}
	return hi < hj
}
