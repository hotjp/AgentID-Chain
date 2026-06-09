package rbac

import (
	"errors"
	"testing"

	"github.com/agentid-chain/agentid-chain/internal/domain"
)

func TestNewDefaultLevelTemplate(t *testing.T) {
	tpl := NewDefaultLevelTemplate()
	if tpl == nil {
		t.Fatal("nil template")
	}
	// LevelTest: 0x0000_0000_0000_FFFF
	if tpl.Max(domain.LevelTest) != 0xFFFF {
		t.Errorf("LevelTest max = %#x, want 0xFFFF", tpl.Max(domain.LevelTest))
	}
	// LevelBasic: 0x0000_0000_FFFF_FFFF
	if tpl.Max(domain.LevelBasic) != 0xFFFFFFFF {
		t.Errorf("LevelBasic max = %#x, want 0xFFFFFFFF", tpl.Max(domain.LevelBasic))
	}
	// LevelAdvanced: 0x0000_FFFF_FFFF_FFFF
	if tpl.Max(domain.LevelAdvanced) != 0xFFFFFFFFFFFF {
		t.Errorf("LevelAdvanced max = %#x, want 0xFFFFFFFFFFFF", tpl.Max(domain.LevelAdvanced))
	}
	// LevelPro: 0xFFFF_FFFF_FFFF_FFFF
	if tpl.Max(domain.LevelPro) != 0xFFFFFFFFFFFFFFFF {
		t.Errorf("LevelPro max = %#x, want full", tpl.Max(domain.LevelPro))
	}
}

func TestNewLevelTemplate_InvalidLevel(t *testing.T) {
	_, err := NewLevelTemplate(map[domain.LevelType]uint64{
		domain.LevelType(99): 0xFF,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("err should wrap ErrInvalidLevel: %v", err)
	}
}

func TestNewLevelTemplate_Empty(t *testing.T) {
	tpl, err := NewLevelTemplate(map[domain.LevelType]uint64{})
	if err != nil {
		t.Fatalf("NewLevelTemplate: %v", err)
	}
	// 空模板：Max 全部回退到 domain 默认值
	if tpl.Max(domain.LevelBasic) != domain.LevelBasic.DefaultMaxPermissions() {
		t.Error("empty template should fallback to domain default")
	}
}

func TestEngine_Check_BitAllowed(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic: bits [0, 32) — bit 0 允许
	ok, err := e.Check(0, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("bit 0 should be allowed for LevelBasic")
	}
	// bit 31 允许（最后一位）
	ok, _ = e.Check(31, domain.LevelBasic)
	if !ok {
		t.Error("bit 31 should be allowed for LevelBasic")
	}
}

func TestEngine_Check_BitNotAllowed(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic 不允许 bit 32
	ok, err := e.Check(32, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("bit 32 should NOT be allowed for LevelBasic")
	}
}

func TestEngine_Check_BitOutOfRange(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	_, err := e.Check(64, domain.LevelBasic)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrBitOutOfRange) {
		t.Errorf("err should wrap ErrBitOutOfRange: %v", err)
	}
}

func TestEngine_Check_InvalidLevel(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	_, err := e.Check(0, domain.LevelType(99))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("err should wrap ErrInvalidLevel: %v", err)
	}
}

func TestEngine_MustCheck(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	if !e.MustCheck(0, domain.LevelBasic) {
		t.Error("MustCheck failed for valid bit")
	}
	// MustCheck 不 panic 的反例通过其他测试覆盖；panic 路径不强制
}

func TestEngine_CheckMask_AllAllowed(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic 范围内所有 bit
	perms := domain.LevelBasic.DefaultMaxPermissions()
	ok, err := e.CheckMask(perms, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("LevelBasic max should pass CheckMask")
	}
}

func TestEngine_CheckMask_ExceedsLevel(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic 不允许 bit 50
	perms := uint64(1) << 50
	ok, err := e.CheckMask(perms, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("bit 50 should NOT be allowed for LevelBasic")
	}
}

func TestEngine_CheckMask_EmptyPerms(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	ok, err := e.CheckMask(0, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("empty perms should always pass")
	}
}

func TestEngine_AllowedBits(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	bits, err := e.AllowedBits(domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if len(bits) != 32 {
		t.Errorf("LevelBasic should have 32 bits, got %d", len(bits))
	}
	// 第一个应该是 bit 31（倒序）
	if bits[0] != 31 {
		t.Errorf("first bit = %d, want 31", bits[0])
	}
}

func TestEngine_AllowedBits_InvalidLevel(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	_, err := e.AllowedBits(domain.LevelType(99))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEngine_HasAny(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic 范围内有 bit 5
	if !e.HasAny(1<<5, domain.LevelBasic) {
		t.Error("HasAny should be true for bit 5")
	}
	// LevelBasic 范围外有 bit 50
	if e.HasAny(1<<50, domain.LevelBasic) {
		t.Error("HasAny should be false for bit 50")
	}
	// 0
	if e.HasAny(0, domain.LevelBasic) {
		t.Error("HasAny(0) should be false")
	}
}

func TestEngine_HasAll(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	// LevelBasic 范围内 bit 5
	if !e.HasAll(1<<5, domain.LevelBasic) {
		t.Error("HasAll should be true")
	}
	// LevelBasic 范围外 bit 50
	if e.HasAll(1<<50, domain.LevelBasic) {
		t.Error("HasAll should be false for out-of-range")
	}
	// 0
	if !e.HasAll(0, domain.LevelBasic) {
		t.Error("HasAll(0) should be true")
	}
}

func TestEngine_MaxPermissions(t *testing.T) {
	e := NewEngine(NewDefaultLevelTemplate())
	if e.MaxPermissions(domain.LevelBasic) != 0xFFFFFFFF {
		t.Error("MaxPermissions mismatch")
	}
}

func TestEngine_NilTemplate(t *testing.T) {
	e := NewEngine(nil)
	// nil 模板：使用 domain 默认
	if e.MaxPermissions(domain.LevelBasic) != domain.LevelBasic.DefaultMaxPermissions() {
		t.Error("nil template should use domain default")
	}
	ok, err := e.CheckMask(0xFFFFFFFF, domain.LevelBasic)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("CheckMask with nil template should pass for valid perms")
	}
}

func TestLevelTemplate_Set(t *testing.T) {
	tpl := NewDefaultLevelTemplate()
	if err := tpl.Set(domain.LevelBasic, 0xFF); err != nil {
		t.Fatal(err)
	}
	if tpl.Max(domain.LevelBasic) != 0xFF {
		t.Error("Set failed")
	}
}

func TestLevelTemplate_Set_InvalidLevel(t *testing.T) {
	tpl := NewDefaultLevelTemplate()
	if err := tpl.Set(domain.LevelType(99), 0xFF); err == nil {
		t.Fatal("expected error")
	}
}

func TestLevelTemplate_Set_NilTemplate(t *testing.T) {
	var tpl *LevelTemplate
	if err := tpl.Set(domain.LevelBasic, 0xFF); !errors.Is(err, ErrTemplateNotSet) {
		t.Errorf("err = %v, want ErrTemplateNotSet", err)
	}
}

func TestLevelTemplate_Snapshot(t *testing.T) {
	tpl := NewDefaultLevelTemplate()
	snap := tpl.Snapshot()
	if len(snap) != int(domain.MaxLevel)+1 {
		t.Errorf("snapshot size = %d, want %d", len(snap), int(domain.MaxLevel)+1)
	}
	// 验证值一致
	if snap[domain.LevelBasic] != 0xFFFFFFFF {
		t.Error("snapshot value mismatch")
	}
}

func TestLevelTemplate_Snapshot_Nil(t *testing.T) {
	var tpl *LevelTemplate
	if tpl.Snapshot() != nil {
		t.Error("nil template snapshot should be nil")
	}
}

func TestLevelTemplate_Max_NilTemplate(t *testing.T) {
	var tpl *LevelTemplate
	if tpl.Max(domain.LevelBasic) != domain.LevelBasic.DefaultMaxPermissions() {
		t.Error("nil template Max should fallback to domain default")
	}
}
