package monger

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// convertToFuzzyFilter converte valores string em regex case-insensitive
// para permitir buscas parciais e tolerantes a erros.
func convertToFuzzyFilter(filter M) M {
	result := M{}
	for k, v := range filter {
		switch val := v.(type) {
		case string:
			// Converte strings em regex case-insensitive para busca fuzzy
			// Escapa caracteres especiais de regex e permite match parcial
			escaped := escapeRegex(val)
			result[k] = primitive.Regex{Pattern: escaped, Options: "i"}
		case M:
			// Recursivamente processa sub-documentos, mas não converte operadores
			if isOperatorDocument(val) {
				result[k] = val
			} else {
				result[k] = convertToFuzzyFilter(val)
			}
		case []M:
			// Para $and, $or, etc.
			converted := make([]M, len(val))
			for i, item := range val {
				converted[i] = convertToFuzzyFilter(item)
			}
			result[k] = converted
		default:
			result[k] = v
		}
	}
	return result
}

// isOperatorDocument verifica se um documento M contém operadores MongoDB ($gt, $lt, etc.)
func isOperatorDocument(m M) bool {
	for k := range m {
		if strings.HasPrefix(k, "$") {
			return true
		}
	}
	return false
}

// escapeRegex escapa caracteres especiais de regex
func escapeRegex(s string) string {
	specialChars := []string{"\\", ".", "+", "*", "?", "^", "$", "(", ")", "[", "]", "{", "}", "|", "-"}
	result := s
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}
	return result
}
