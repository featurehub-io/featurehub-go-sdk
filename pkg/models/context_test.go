package models

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContext(t *testing.T) {

	// Make a new context:
	context := &Context{
		Userkey:  "some-random-string",
		Session:  "some-session-ID",
		Device:   "desktop",
		Platform: "macos",
		Country:  "new_zealand",
		Version:  "5.0.0",
	}

	assert.Equal(t, url.QueryEscape("userkey=some-random-string,session=some-session-ID,device=desktop,platform=macos,country=new_zealand,version=5.0.0"), context.String())
}

// --- GenerateHeader ---

func TestGenerateHeaderEmptyContext(t *testing.T) {
	c := &Context{}
	assert.Equal(t, "", c.GenerateHeader())
}

func TestGenerateHeaderUserkeyOnly(t *testing.T) {
	c := &Context{Userkey: "alice"}
	assert.Equal(t, "userkey=alice", c.GenerateHeader())
}

func TestGenerateHeaderSessionOnly(t *testing.T) {
	c := &Context{Session: "sess-123"}
	assert.Equal(t, "session=sess-123", c.GenerateHeader())
}

func TestGenerateHeaderDeviceOnly(t *testing.T) {
	c := &Context{Device: ContextDeviceMobile}
	assert.Equal(t, "device=mobile", c.GenerateHeader())
}

func TestGenerateHeaderPlatformOnly(t *testing.T) {
	c := &Context{Platform: ContextPlatformLinux}
	assert.Equal(t, "platform=linux", c.GenerateHeader())
}

func TestGenerateHeaderCountryOnly(t *testing.T) {
	c := &Context{Country: ContextCountryThailand}
	assert.Equal(t, "country=thailand", c.GenerateHeader())
}

func TestGenerateHeaderVersionOnly(t *testing.T) {
	c := &Context{Version: "2.3.4"}
	assert.Equal(t, "version=2.3.4", c.GenerateHeader())
}

func TestGenerateHeaderAllStandardFieldsSorted(t *testing.T) {
	c := &Context{
		Userkey:  "bob",
		Session:  "s1",
		Device:   ContextDeviceDesktop,
		Platform: ContextPlatformMacos,
		Country:  ContextCountryNewZealand,
		Version:  "1.0.0",
	}
	// Parts sorted alphabetically: country, device, platform, session, userkey, version
	expected := "country=new_zealand&device=desktop&platform=macos&session=s1&userkey=bob&version=1.0.0"
	assert.Equal(t, expected, c.GenerateHeader())
}

func TestGenerateHeaderCustomAttributeOnly(t *testing.T) {
	c := &Context{Custom: map[string]interface{}{"tier": "gold"}}
	assert.Equal(t, "tier=gold", c.GenerateHeader())
}

func TestGenerateHeaderCustomAttributeMixedWithStandard(t *testing.T) {
	c := &Context{
		Userkey: "carol",
		Custom:  map[string]interface{}{"plan": "pro"},
	}
	// sorted: plan, userkey
	assert.Equal(t, "plan=pro&userkey=carol", c.GenerateHeader())
}

func TestGenerateHeaderURLEncodesUserkey(t *testing.T) {
	c := &Context{Userkey: "user name+special"}
	assert.Equal(t, "userkey=user+name%2Bspecial", c.GenerateHeader())
}

func TestGenerateHeaderURLEncodesVersion(t *testing.T) {
	c := &Context{Version: "1.0.0-beta+build"}
	assert.Equal(t, "version=1.0.0-beta%2Bbuild", c.GenerateHeader())
}

func TestGenerateHeaderURLEncodesCustomValue(t *testing.T) {
	c := &Context{Custom: map[string]interface{}{"note": "hello world"}}
	assert.Equal(t, "note=hello+world", c.GenerateHeader())
}

func TestGenerateHeaderCustomNonStringValueFormattedAsString(t *testing.T) {
	c := &Context{Custom: map[string]interface{}{"count": float64(42)}}
	assert.Equal(t, "count=42", c.GenerateHeader())
}
