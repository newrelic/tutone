package schema

// https://github.com/graphql/graphql-js/blob/master/src/utilities/getIntrospectionQuery.js#L35
//
//	Modified from the following as we only care about the Types
//	query IntrospectionQuery {
//	  __schema {
//	    directives { name description locations args { ...InputValue } }
//	    mutationType { name }
//	    queryType { name }
//	    subscriptionType { name }
//	    types { ...FullType }
//	  }
//	}
const (
	// QuerySchema gets basic info about the schema
	QuerySchema = `query { __schema { mutationType { name } queryType { name } subscriptionType { name } } }`

	// QuerySchemaTypes is used to fetch all of the data types in the schema
	QuerySchemaTypes = `query { __schema { types { ...FullType } } }` + " " + fragmentFullType

	// QueryType returns all of the data on that specific type, useful for fetching queries / mutations
	QueryType = `query($typeName:String!) { __type(name: $typeName) { ...FullType } }` + " " + fragmentFullType

	// Reusable query fragments
	fragmentFullType = `
fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) { name description args { ...InputValue } type { ...TypeRef } isDeprecated deprecationReason }
  inputFields { ...InputValue }
  interfaces { ...TypeRef }
  enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason }
  possibleTypes { ...TypeRef }
}` + " " + fragmentInputValue + " " + fragmentTypeRef

	fragmentInputValue = `fragment InputValue on __InputValue { name description type { ...TypeRef } defaultValue }`
	fragmentTypeRef    = `fragment TypeRef on __Type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } }`
)

// Helper function to make queries, lives here with the constant query
type QueryTypeVars struct {
	Name string `json:"typeName"`
}

type QueryResponse struct {
	Data struct {
		Type   Type   `json:"__type"`
		Schema Schema `json:"__schema"`
	} `json:"data"`
}
