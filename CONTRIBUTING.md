## Contributing

Contributions are welcome! Please open issues or submit pull requests.

### Arena Safety (Important)

If you plan to modify or extend memory allocation strategies, please
read the "Arena Safety Note" in `README.md` before using the raw
byte-backed `Arena` implemented in `arena.go`.

Summary:
- The raw byte-buffer `Arena` is suitable only for non-pointer, raw
  byte payloads (C-style POD data).
- Do NOT allocate Go structs that contain pointers (slices, maps,
  interfaces) in the raw `Arena` — the Go garbage collector will not
  scan pointers stored in `[]byte`, which can lead to crashes or memory
  corruption.

Recommendation: use the typed, GC-safe chunk allocator used for nodes
in `node.go` (see `arenaAllocator`), or follow the guidance in the
README.
