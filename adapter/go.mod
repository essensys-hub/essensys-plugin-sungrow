module github.com/essensys-hub/essensys-plugin-sungrow/adapter

go 1.24

require github.com/essensys-hub/essensys-plugin-framework/go v0.0.0

// Dev local (layout monorepo-parent). En CI, remplacer par une version taguée.
replace github.com/essensys-hub/essensys-plugin-framework/go => ../../essensys-plugin-framework/go
