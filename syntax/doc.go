// Package syntax is the public syntax tree of a Markdown document, as the
// editor reads it: node kinds, source ranges, the bytes of each node's
// syntax, and the element each node is for a theme's selectors.
//
// A Tree is a snapshot of one source string, and read-only: it does not
// change as the editor's source does. Node is a value, holding a node of
// the tree; a Tree's nodes are built once, when it is parsed.
//
// The element vocabulary is documented in the design (§3.3) and in
// theme's element table: selectors in theme sheets name the same types,
// classes and attributes.
//
// A tree is built in full when it is parsed: its nodes do not change after
// that. Code tokens are not in it, as the editor makes them as it
// highlights; a theme styles them where it draws them.
package syntax
