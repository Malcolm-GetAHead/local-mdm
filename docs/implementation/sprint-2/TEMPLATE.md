# Implementation Template

Use this template when creating implementation documents for Sprint 2 issues.

## File Naming Convention

Place implementation documents in the appropriate priority directory:
- `docs/implementation/sprint-2/critical/[ISSUE-ID]-[short-name].md`
- `docs/implementation/sprint-2/high/[ISSUE-ID]-[short-name].md`
- `docs/implementation/sprint-2/medium/[ISSUE-ID]-[short-name].md`
- `docs/implementation/sprint-2/low/[ISSUE-ID]-[short-name].md`

Examples:
- `docs/implementation/sprint-2/critical/C-01-fix-body-reading.md`
- `docs/implementation/sprint-2/high/H-03-audit-logging.md`

---

## Template Structure

```markdown
# [Issue ID]: [Issue Title]

**Priority**: [CRITICAL/HIGH/MEDIUM/LOW]
**Status**: [Open/In Progress/In Review/Done]
**Assignee**: [Your Name]
**Estimated Effort**: [X hours/days]
**Actual Effort**: [X hours/days]
**Started**: [YYYY-MM-DD]
**Completed**: [YYYY-MM-DD]

## Problem Statement

[Brief description of the issue from the review document]

**Affected Files**:
- `path/to/file.go:line-numbers`
- `path/to/another/file.go:line-numbers`

**Impact**:
- [Security/Reliability/Performance impact]
- [What breaks if not fixed]

## Solution Approach

[High-level description of how you plan to fix it]

**Key Changes**:
1. [Change 1]
2. [Change 2]
3. [Change 3]

**Dependencies**:
- [ ] [Any prerequisite issues that must be fixed first]
- [ ] [Any new packages or tools needed]

## Implementation Details

### Files Modified

#### `path/to/file.go`
**Before**:
```go
// Old code
```

**After**:
```go
// New code
```

**Rationale**: [Why this change was made]

### Files Added

#### `path/to/new/file.go`
```go
// New file content
```

**Purpose**: [Why this file was added]

### Configuration Changes

If configuration changes are needed:

#### `configs/config.example.yaml`
```yaml
# New configuration
```

### Database Changes

If database migrations are needed:

#### `migrations/YYYYMMDDHHMMSS_description.up.sql`
```sql
-- Migration SQL
```

## Testing

### Unit Tests Added

#### `path/to/file_test.go`
```go
func TestNewFeature(t *testing.T) {
    // Test implementation
}
```

**Coverage**: [X% coverage achieved]

### Integration Tests

[Description of integration tests added]

### Manual Testing

**Test Scenarios**:
1. [Scenario 1]
   - Steps: [...]
   - Expected: [...]
   - Actual: [...]
   
2. [Scenario 2]
   - Steps: [...]
   - Expected: [...]
   - Actual: [...]

### Performance Testing

[If applicable, performance test results]

## Verification

- [ ] All tests pass (`go test ./...`)
- [ ] Race detector clean (`go test -race ./...`)
- [ ] Coverage improved (`go test -cover ./...`)
- [ ] No vet warnings (`go vet ./...`)
- [ ] Manual testing complete
- [ ] Documentation updated
- [ ] Code reviewed
- [ ] Security review (for security-related changes)

## Verification Commands

```bash
# Run tests
go test -v ./path/to/package

# Run with race detector
go test -race ./path/to/package

# Check coverage
go test -cover ./path/to/package

# Run vet
go vet ./path/to/package

# Run specific test
go test -v -run TestSpecificTest ./path/to/package
```

## Related Issues

- Fixes: [Issue ID from review]
- Related: [Any related issues]
- Blocks: [Issues that depend on this]
- Blocked by: [Issues this depends on]

## Documentation Updates

- [ ] Updated README if needed
- [ ] Updated API documentation
- [ ] Updated configuration documentation
- [ ] Updated deployment documentation
- [ ] Added code comments

## Rollback Plan

[How to rollback this change if needed]

**Rollback Steps**:
1. [Step 1]
2. [Step 2]

## Notes

[Any additional notes, considerations, or lessons learned]

### Challenges Encountered

[Any challenges faced during implementation]

### Alternative Approaches Considered

[Other approaches considered and why they were not chosen]

### Future Improvements

[Any future improvements that could be made]

## Sign-off

**Developer**: [Name] - [Date]
**Reviewer**: [Name] - [Date]
**QA**: [Name] - [Date]
```

---

## Example Implementation Document

See `docs/implementation/sprint-1/` for examples of completed implementation documents from Sprint 1.

## Workflow

1. **Pick an issue** from `docs/reviews/sprint-2/ISSUE_TRACKING.md`
2. **Create implementation document** using this template
3. **Update status** to "In Progress" in ISSUE_TRACKING.md
4. **Implement the fix** following the solution approach
5. **Add tests** to verify the fix
6. **Update documentation** as needed
7. **Submit PR** with reference to implementation document
8. **Update status** to "In Review" in ISSUE_TRACKING.md
9. **After merge**, update status to "Done" with completion date
10. **Update progress** in README.md

## Tips

- Keep implementation documents up-to-date as you work
- Document challenges and decisions for future reference
- Include actual code snippets, not just descriptions
- Add verification steps that others can follow
- Link to related issues and PRs
- Update the issue tracking document regularly
