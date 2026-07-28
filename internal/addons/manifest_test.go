package addons_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apikcloud/odoo-builder/internal/addons"
)

func TestParseManifest_SaleCustomFixture(t *testing.T) {
	manifest, err := addons.ParseManifest("../../testdata/simple/addons/sale_custom/__manifest__.py")
	require.NoError(t, err)

	assert.Equal(t, "Sale Custom", manifest["name"])
	assert.Equal(t, "18.0.1.0.0", manifest["version"])
	assert.Equal(t, []interface{}{"sale"}, manifest["depends"])
	assert.Equal(t, true, manifest["installable"])
}

func TestParseManifest_ValidVariants(t *testing.T) {
	manifest := writeAndParse(t, `
{
    # top-level comment
    'name': 'Full Manifest',
    'summary': None,
    'author': "Example",  # trailing comment
    'sequence': -12,
    'version': 1.5,
    'metadata': {
        'nested': True,
        'deep': {'level': 3},
    },
    'depends': ['base', 'sale'],
}
`)

	assert.Equal(t, "Full Manifest", manifest["name"])
	assert.Nil(t, manifest["summary"])
	assert.Equal(t, "Example", manifest["author"])
	assert.Equal(t, -12.0, manifest["sequence"])
	assert.Equal(t, 1.5, manifest["version"])

	metadata, ok := manifest["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, metadata["nested"])

	deep, ok := metadata["deep"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 3.0, deep["level"])

	assert.Equal(t, []interface{}{"base", "sale"}, manifest["depends"])
}

func TestParseManifest_AdjacentStringLiterals_Concatenated(t *testing.T) {
	manifest := writeAndParse(t, `
{
    'name': 'Multi Author Module',
    'author': "Compassion CH, "
    "Tecnativa, "
    "Odoo Community Association (OCA)",
}
`)

	assert.Equal(t, "Compassion CH, Tecnativa, Odoo Community Association (OCA)", manifest["author"])
}

func TestParseManifest_AdjacentStringLiteralKeys_Concatenated(t *testing.T) {
	manifest := writeAndParse(t, `
{
    'na' 'me': 'Split Key Module',
}
`)

	assert.Equal(t, "Split Key Module", manifest["name"])
}

func TestParseManifest_TripleQuotedDescription_Parsed(t *testing.T) {
	manifest := writeAndParse(t, `
{
    "name": "Andermatt - Partner",
    "description": """
        Andermatt - Partner
    """,
    "author": "Apik - THE",
}
`)

	assert.Equal(t, "Andermatt - Partner", manifest["name"])
	assert.Equal(t, "\n        Andermatt - Partner\n    ", manifest["description"])
	assert.Equal(t, "Apik - THE", manifest["author"])
}

func TestParseManifest_TripleQuotedDescription_ContainsUnescapedSingleQuote(t *testing.T) {
	manifest := writeAndParse(t, `
{
    "name": "Module",
    "description": '''
        It's a multi-line "description" with quotes.
    ''',
}
`)

	assert.Equal(t, "\n        It's a multi-line \"description\" with quotes.\n    ", manifest["description"])
}

func TestParseManifest_AssetRemoveTuple_ParsedAsList(t *testing.T) {
	manifest := writeAndParse(t, `
{
    'name': 'Accounting',
    'assets': {
        'web.assets_unit_tests': [
            'account_accountant/static/tests/**/*',
            ('remove', 'account_accountant/static/tests/tours/**/*'),
        ],
    },
}
`)

	assets, ok := manifest["assets"].(map[string]interface{})
	require.True(t, ok)
	tests, ok := assets["web.assets_unit_tests"].([]interface{})
	require.True(t, ok)
	require.Len(t, tests, 2)
	assert.Equal(t, "account_accountant/static/tests/**/*", tests[0])
	assert.Equal(t, []interface{}{"remove", "account_accountant/static/tests/tours/**/*"}, tests[1])
}

func TestParseManifest_RawTripleQuotedDescription_BackslashesPreservedLiterally(t *testing.T) {
	manifest := writeAndParse(t, `
{
    'name': 'Import CSV Bank Statement',
    'description': r'''
Accounting \ Bank and Cash \ Bank Statements.
''',
}
`)

	assert.Equal(t, "\nAccounting \\ Bank and Cash \\ Bank Statements.\n", manifest["description"])
}

func TestParseManifest_BackslashNewlineContinuation_ProducesNoCharacter(t *testing.T) {
	manifest := writeAndParse(t, "{\n    'name': 'X',\n    'description': 'line one \\\ncontinues here',\n}\n")

	assert.Equal(t, "line one continues here", manifest["description"])
}

func TestParseManifest_UnterminatedDict_ReturnsError(t *testing.T) {
	_, err := parseString(t, `{'name': 'Broken'`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid manifest")
}

func TestParseManifest_NonDictTopLevel_ReturnsError(t *testing.T) {
	_, err := parseString(t, `['not', 'a', 'dict']`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a dict")
}

func TestParseManifest_BareStringTopLevel_ReturnsError(t *testing.T) {
	_, err := parseString(t, `'just a string'`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a dict")
}

func TestParseManifest_UnterminatedString_ReturnsError(t *testing.T) {
	_, err := parseString(t, `{'name': 'Broken}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid manifest")
}

func TestParseManifest_TrailingGarbage_ReturnsError(t *testing.T) {
	_, err := parseString(t, `{'name': 'OK'} garbage`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing content")
}

func writeAndParse(t *testing.T, content string) map[string]interface{} {
	t.Helper()
	manifest, err := parseString(t, content)
	require.NoError(t, err)
	return manifest
}

func parseString(t *testing.T, content string) (map[string]interface{}, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "__manifest__.py")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return addons.ParseManifest(path)
}
