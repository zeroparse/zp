package check

import "zeroparse.dev/zp/internal/types"

// The type model lives in internal/types. The checker names it unqualified through these aliases.

type (
	Ty       = types.Ty
	BasicTy  = types.BasicTy
	NamedTy  = types.NamedTy
	ListTy   = types.ListTy
	OptTy    = types.OptTy
	JoinTy   = types.JoinTy
	RecordTy = types.RecordTy
	TupleTy  = types.TupleTy
	CapIndex = types.CapIndex
	NodeTy   = types.NodeTy
	RuleTy   = types.RuleTy
)

const (
	TyAny    = types.TyAny
	TyInt    = types.TyInt
	TyString = types.TyString
	TyBool   = types.TyBool
	TyNil    = types.TyNil
	TyNode   = types.TyNode
	TyTop    = types.TyTop
	TyBottom = types.TyBottom
	TyErr    = types.TyErr
)

var (
	TyMatch      = types.TyMatch
	TyError      = types.TyError
	TyMissing    = types.TyMissing
	TyStringNode = types.TyStringNode
)

// Vjoin is types.Vjoin.
func Vjoin(ts ...Ty) Ty { return types.Vjoin(ts...) }

// IsNode is types.IsNode.
func IsNode(t Ty) bool { return types.IsNode(t) }

// FromLegacy is types.FromLegacy.
func FromLegacy(t Ty) Ty { return types.FromLegacy(t) }

// FieldType is types.FieldType.
type FieldType = types.FieldType

// NewFieldType is types.NewFieldType.
func NewFieldType(t Ty) *FieldType { return types.NewFieldType(t) }
