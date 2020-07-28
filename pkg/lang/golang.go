package lang

// GolangGenerator is enough information to generate Go code for a single package.
type GolangGenerator struct {
	Types       []GoStruct
	PackageName string
	Enums       []GoEnum
	Imports     []string
	Scalars     []GoScalar
	Interfaces  []GoInterface
}

type GoStruct struct {
	Name        string
	Description string
	Fields      []GoStructField
}

type GoStructField struct {
	Name        string
	Type        string
	Tags        string
	Description string
}

type GoEnum struct {
	Name        string
	Description string
	Values      []GoEnumValue
}

type GoEnumValue struct {
	Name        string
	Description string
}

type GoScalar struct {
	Name        string
	Description string
	Type        string
}

type GoInterface struct {
	Name        string
	Description string
	Type        string
}
