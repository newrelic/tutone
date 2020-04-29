package schema

type Kind string

const (
	KindENUM        Kind = "ENUM"
	KindInputObject Kind = "INPUT_OBJECT"
	KindInterface   Kind = "INTERFACE"
	KindList        Kind = "LIST"
	KindNonNull     Kind = "NON_NULL"
	KindObject      Kind = "OBJECT"
	KindScalar      Kind = "SCALAR"
)
