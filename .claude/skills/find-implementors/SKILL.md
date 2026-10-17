---
name: find-implementors
description: Find all types in this Go codebase that implement a given interface. Useful after adding a method to an interface to locate every type that needs updating. Usage: /find-implementors InterfaceName
user-invocable: true
argument-hint: <InterfaceName>
allowed-tools: Grep, Glob, Read, Bash
---

Find every Go type in this codebase that implements the interface `$ARGUMENTS`.

Steps:
1. Locate the interface definition — search for `type $ARGUMENTS interface` across all `.go` files.
2. Read the interface to extract the full list of method signatures.
3. Search the codebase for types that define all of those methods (search for each method name as a receiver method: `func (.*) MethodName(`).
4. For each candidate type found, confirm it implements the full interface by checking all required methods are present.
5. Report:
   - The interface definition (file + line)
   - Each implementing type (file, type name, and whether it is a pointer or value receiver)
   - Any types that appear partial (have some methods but not all) — flag these as likely needing updates.

Be thorough: check `_test.go` files too, as test mocks frequently implement interfaces and are easy to miss.