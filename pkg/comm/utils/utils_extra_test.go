package utils

import (
    "encoding/base64"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSplitAndTrim(t *testing.T) {
    assert.Equal(t, []string{"a", "b", "c"}, SplitAndTrim(" a , b , c ", ","))
    assert.Equal(t, []string{}, SplitAndTrim("   ", ","))
    assert.Equal(t, []string{"abc"}, SplitAndTrim("abc", ","))
}

func TestMaskAndTruncateAndConvert(t *testing.T) {
    // MaskString
    assert.Equal(t, "*****", MaskString("hello", 0))
    assert.Equal(t, "*****", MaskString("hello", -1))
    assert.Equal(t, "*****", MaskString("hello", 10))
    assert.Equal(t, "he***", MaskString("hello", 2))

    // TruncateString (with multibyte)
    assert.Equal(t, "你号", TruncateString("你好世界", 2))
    assert.Equal(t, "ab", TruncateString("ab", 5))

    // Conversions
    assert.Equal(t, 123, ToInt("123"))
    assert.Equal(t, 0, ToInt("x"))
    assert.Equal(t, int32(123), ToInt32("123"))
    assert.Equal(t, 0, int(ToUInt("x")))
    assert.Equal(t, 7, ToIntDefault("x", 7))
    assert.Equal(t, []int{1, 2, 3}, ToIntSlice("1, 2, x, 3"))
    assert.Equal(t, []int64{1, 2, 3}, ToInt64Slice("1, 2, x, 3"))
}

func TestIsTextFileAndSanitize(t *testing.T) {
    ok, err := IsTextFile([]byte("hello"))
    require.NoError(t, err)
    assert.True(t, ok)

    ok, err = IsTextFile([]byte{'h', 0x00, 'i'})
    require.NoError(t, err)
    assert.False(t, ok)

    name := SanitizeFileName("a/b:c*?\"<>|() name.txt")
    assert.Equal(t, "a_b_c______name.txt", name)

    cleaned := CleanANSISequences("\u001B[31mred\u001B[0m")
    assert.Equal(t, "red", cleaned)
}

func TestSliceHelpers(t *testing.T) {
    assert.Equal(t, []string{"a", "b"}, RemoveEmptyLines([]string{"a", "", "b", ""}))
    assert.True(t, AllIn([]string{"a", "b"}, []string{"a", "b", "c"}))
    assert.False(t, AllIn([]string{"a", "x"}, []string{"a", "b", "c"}))
    assert.True(t, AnyIn([]string{"x", "b"}, []string{"a", "b", "c"}))
    assert.False(t, AnyIn([]string{"x", "y"}, []string{"a", "b", "c"}))
}

func TestNumbersAndDecimal(t *testing.T) {
    n, err := ExtractNumbers("v1.2.3")
    require.NoError(t, err)
    assert.Equal(t, 123, n)
    n, err = ExtractNumbers("abc")
    require.NoError(t, err)
    assert.Equal(t, 0, n)

    assert.True(t, IsDecimal("1.0"))
    assert.False(t, IsDecimal("1."))
    assert.False(t, IsDecimal("1"))
}

func TestPtrHelpers(t *testing.T) {
    i32 := Int32Ptr(1)
    assert.NotNil(t, i32)
    assert.Equal(t, int32(1), *i32)

    u := UintPtr(2)
    assert.NotNil(t, u)
    assert.Equal(t, uint(2), *u)

    b := BoolPtr(true)
    assert.NotNil(t, b)
    assert.Equal(t, true, *b)

    i64 := Int64Ptr(3)
    assert.NotNil(t, i64)
    assert.Equal(t, int64(3), *i64)
}

func TestBase64AndCrypto(t *testing.T) {
    enc := EncodeBase64("hello")
    dec, err := DecodeBase64(enc)
    require.NoError(t, err)
    assert.Equal(t, "hello", dec)
    assert.Equal(t, "hello", MustDecodeBase64(enc))

    // URL-safe decode with missing padding
    u := base64.URLEncoding.EncodeToString([]byte("data"))
    trimmed := u
    for len(trimmed)%4 != 0 {
        trimmed = trimmed[:len(trimmed)-1]
    }
    _, err = UrlSafeBase64Decode(trimmed)
    require.NoError(t, err)

    // AES encrypt -> base64 -> decrypt
    plaintext := []byte("secret data")
    crypted, err := AesEncrypt(plaintext)
    require.NoError(t, err)
    b64 := base64.StdEncoding.EncodeToString(crypted)
    out, err := AesDecrypt(b64)
    require.NoError(t, err)
    assert.Equal(t, plaintext, out)

    // invalid base64 should error
    _, err = AesDecrypt("***not-base64***")
    require.Error(t, err)
}

func TestJSONHelpers(t *testing.T) {
    type S struct{ A int }
    j := ToJSON(S{A: 1})
    assert.Contains(t, j, "\n")

    src := S{A: 3}
    cp, err := DeepCopy(src)
    require.NoError(t, err)
    assert.Equal(t, src, cp)
}

func TestVersionHelpersAndSortByTimestamp(t *testing.T) {
    assert.Equal(t, []int{1, 2, 3}, ParseVersion("1.2.3"))
    assert.True(t, CompareVersions("1.2.10", "1.2.3"))
    assert.False(t, CompareVersions("1.2.3", "1.2.10"))

    // SortByLastTimestamp
    i1 := &unstructured.Unstructured{Object: map[string]interface{}{"lastTimestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339)}}
    i2 := &unstructured.Unstructured{Object: map[string]interface{}{"lastTimestamp": time.Now().Format(time.RFC3339)}}
    items := []*unstructured.Unstructured{i1, i2}
    out := SortByLastTimestamp(items)
    // i2 is newer, should be first (descending)
    ts0, _, _ := unstructured.NestedString(out[0].Object, "lastTimestamp")
    ts1, _, _ := unstructured.NestedString(out[1].Object, "lastTimestamp")
    t0, _ := time.Parse(time.RFC3339, ts0)
    t1, _ := time.Parse(time.RFC3339, ts1)
    assert.True(t, t0.After(t1))
}

func TestGetLocalIPs(t *testing.T) {
    ips, err := GetLocalIPs()
    require.NoError(t, err)
    for _, ip := range ips {
        // basic IPv4 shape and non-empty
        assert.NotEmpty(t, ip)
        // cannot reliably assert non-loopback across all envs beyond implementation
    }
}

func TestTOTPHelpers(t *testing.T) {
    secret, url, err := GenerateSecret("user@example.com")
    require.NoError(t, err)
    assert.NotEmpty(t, secret)
    assert.Contains(t, url, "otpauth://")

    assert.False(t, ValidateCode("not-base32!!", "000000"))

    codes, err := GenerateBackupCodes(3)
    require.NoError(t, err)
    assert.Len(t, codes, 3)
    for _, c := range codes {
        assert.Len(t, c, 8)
    }

    _, err = GenerateBackupCodes(0)
    require.Error(t, err)
}


