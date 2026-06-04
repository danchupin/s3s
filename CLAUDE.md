<!-- SPECKIT START -->
Active feature: 001-s3-readonly-browser (read-only S3 TUI browser).

Read the current plan and design artifacts for tech, structure, and commands:
- specs/001-s3-readonly-browser/plan.md       (tech context, constitution check, structure)
- specs/001-s3-readonly-browser/research.md    (library decisions)
- specs/001-s3-readonly-browser/data-model.md  (entities)
- specs/001-s3-readonly-browser/contracts/     (storage interface, config schema, TUI contract)
- specs/001-s3-readonly-browser/quickstart.md  (setup + run + tests)

Stack: Go 1.24, aws-sdk-go-v2, charmbracelet/bubbletea v2 (+bubbles, lipgloss),
go.yaml.in/yaml/v3, log/slog, testcontainers-go (MinIO) for integration tests.
Constitution v1.0.0 governs: core/UI separation, non-blocking TUI, TDD (non-negotiable),
real-backend integration tests, read-only by construction.
<!-- SPECKIT END -->
