# Agent Assignment Guide

**Purpose**: Quick reference for assigning work packages to agentic coding agents

## Current Sprint: Sprint 1 - Foundation

**Duration**: Weeks 1-2  
**Agents Needed**: 3  
**Status**: Ready to start

---

## 🔵 Agent 1: Authentication Agent

**Work Package**: WP1 - Authentication & Authorization System  
**Priority**: CRITICAL  
**Duration**: 1-2 weeks  
**Branch**: `feature/wp1-authentication`

### Context Files to Read
1. `docs/TASK_BREAKDOWN.md` - Section "WORK PACKAGE 1"
2. `docs/DATABASE.md` - Users and API tokens tables
3. `docs/API.md` - Auth endpoints
4. `internal/models/models.go` - User and APIToken models

### Tasks Summary
- Implement JWT token generation and validation
- Create authentication middleware
- Add password hashing (bcrypt)
- Implement RBAC system
- Create API token management
- Update auth-related API handlers

### Files to Create
- `internal/auth/jwt.go`
- `internal/auth/middleware.go`
- `internal/auth/tokens.go`
- `internal/auth/password.go`
- `internal/auth/rbac.go`
- `internal/auth/permissions.go`
- `internal/auth/api_tokens.go`

### Files to Update
- `internal/api/handlers.go` (implement handleLogin, handleRefresh)
- `internal/api/server.go` (add auth middleware to routes)

### Success Criteria
- [ ] User can login and receive JWT token
- [ ] Protected endpoints require valid token
- [ ] RBAC enforces role-based access
- [ ] API tokens work for automation
- [ ] Unit tests: 80%+ coverage
- [ ] Integration tests pass

### Getting Started
```bash
cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
git checkout -b feature/wp1-authentication
# Read context files above
# Start with internal/auth/jwt.go
```

---

## 🟢 Agent 2: Data Agent

**Work Package**: WP2 - Repository & Service Layer  
**Priority**: CRITICAL  
**Duration**: 1-2 weeks  
**Branch**: `feature/wp2-data-layer`

### Context Files to Read
1. `docs/TASK_BREAKDOWN.md` - Section "WORK PACKAGE 2"
2. `docs/DATABASE.md` - All tables
3. `docs/ARCHITECTURE.md` - Repository and Service layers
4. `internal/models/models.go` - All models
5. `internal/db/db.go` - Database connection

### Tasks Summary
- Implement repository pattern for all entities
- Create service layer with business logic
- Add transaction support
- Build query helpers (pagination, filtering, sorting)
- Write comprehensive tests

### Files to Create
- `internal/repository/repository.go`
- `internal/repository/user.go`
- `internal/repository/device.go`
- `internal/repository/policy.go`
- `internal/repository/certificate.go`
- `internal/repository/audit.go`
- `internal/service/user.go`
- `internal/service/device.go`
- `internal/service/policy.go`
- `internal/service/enrollment.go`
- `internal/service/command.go`
- `internal/db/transaction.go`
- `internal/db/query.go`

### Files to Update
- `internal/api/handlers.go` (use services instead of direct DB)
- `internal/api/server.go` (inject services)

### Success Criteria
- [ ] All repositories implement CRUD operations
- [ ] Services contain business logic
- [ ] Transactions work correctly
- [ ] Query helpers support pagination/filtering
- [ ] Unit tests: 70%+ coverage
- [ ] Integration tests with real database

### Getting Started
```bash
cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
git checkout -b feature/wp2-data-layer
# Read context files above
# Start with internal/repository/repository.go (base interface)
```

---

## 🟡 Agent 3: Security Agent

**Work Package**: WP3 - Certificate Infrastructure  
**Priority**: HIGH  
**Duration**: 1-2 weeks  
**Branch**: `feature/wp3-certificates`

### Context Files to Read
1. `docs/TASK_BREAKDOWN.md` - Section "WORK PACKAGE 3"
2. `docs/DATABASE.md` - Certificates table
3. `docs/SCOPE.md` - Certificate requirements
4. `internal/models/models.go` - Certificate model

### Tasks Summary
- Generate and manage root CA certificate
- Implement device certificate signing
- Create certificate revocation list (CRL)
- Handle APNs certificates for macOS
- Add certificate API endpoints

### Files to Create
- `internal/certs/ca.go`
- `internal/certs/storage.go`
- `internal/certs/device.go`
- `internal/certs/signing.go`
- `internal/certs/revocation.go`
- `internal/certs/crl.go`
- `internal/certs/apns.go`

### Files to Update
- `internal/api/handlers.go` (add certificate handlers)
- `internal/api/server.go` (add certificate routes)

### Success Criteria
- [ ] CA certificate generated and stored securely
- [ ] Device certificates can be signed
- [ ] CRL generation works
- [ ] APNs certificate upload/validation
- [ ] Certificate API endpoints functional
- [ ] Unit tests: 80%+ coverage
- [ ] Security documentation updated

### Getting Started
```bash
cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
git checkout -b feature/wp3-certificates
# Read context files above
# Start with internal/certs/ca.go
```

---

## Coordination Between Agents

### Shared Resources
- **Database schema**: Already defined, don't modify
- **Models**: Already defined in `internal/models/models.go`
- **API structure**: Defined in `internal/api/server.go`

### Integration Points

**Agent 1 (Auth) → Agent 2 (Data)**:
- Auth agent creates middleware
- Data agent uses middleware in service layer
- Mock the middleware for testing

**Agent 2 (Data) → Agent 3 (Certs)**:
- Certs agent uses repository layer for storage
- Can stub repository initially for testing
- Integrate once repository is ready

**Agent 3 (Certs) → Agent 1 (Auth)**:
- Auth uses certs for device authentication
- Can stub cert validation initially
- Integrate once cert infrastructure is ready

### Communication Protocol

1. **Daily sync** (optional): Brief status update
2. **Interface contracts**: Define interfaces early, share in PR
3. **Mock dependencies**: Use mocks/stubs for testing
4. **Integration testing**: Test together at sprint end

### Testing Strategy

Each agent should:
1. Write unit tests for their package (70-80% coverage)
2. Mock external dependencies
3. Create integration tests for their package
4. Document any assumptions

At sprint end:
1. Merge all branches
2. Run full integration test suite
3. Fix any integration issues together

---

## Sprint 1 Timeline

### Week 1
- **Day 1-2**: Setup, read context, design interfaces
- **Day 3-5**: Core implementation
- **Day 6-7**: Testing and documentation

### Week 2
- **Day 1-3**: Complete implementation
- **Day 4-5**: Integration testing
- **Day 6-7**: Bug fixes, documentation, PR review

---

## After Sprint 1: Sprint 2 - Platforms

**Agents Needed**: 3  
**Work Packages**:
- WP4: Windows MDM (Agent 1)
- WP5: macOS MDM (Agent 2)
- WP6: Android MDM (Agent 3)

**Dependencies**: Sprint 1 must be complete (Auth, Data, Certs)

---

## Quick Commands for Each Agent

### Start Work
```bash
cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
git pull origin main
git checkout -b feature/wp{N}-{name}
make docker-up  # Start PostgreSQL
make run        # Test server works
```

### During Development
```bash
make test       # Run tests
make build      # Build binary
make fmt        # Format code
```

### Finish Work
```bash
git add .
git commit -m "Implement WP{N}: {description}"
git push origin feature/wp{N}-{name}
# Create PR on GitHub
```

### Update Progress
Edit `docs/PROGRESS.md` and add your completion status:
```markdown
### WP{N} Status
- [x] Task 1
- [x] Task 2
- [ ] Task 3 (in progress)
```

---

## Questions?

Check these files:
- **TASK_BREAKDOWN.md** - Detailed task descriptions
- **ARCHITECTURE.md** - System design
- **DATABASE.md** - Database schema
- **API.md** - API endpoints
- **PROGRESS.md** - Current status

---

**Ready to start?** Assign agents and begin Sprint 1! 🚀
