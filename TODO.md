# TODO

## Completed
- ✅ Update project to Go 1.25
- ✅ Refactor compatible tests to use synctest
- ✅ Successfully refactored `session_test.go` with synctest
- ✅ Documented synctest limitations

## Not Started / Future Work

### Testing Improvements
- [ ] Consider refactoring more unit tests to use synctest (non-I/O, non-signal tests)
- [ ] Add more unit tests that can benefit from synctest's instant time advancement
- [ ] Document best practices for using synctest in TESTING.md

### Potential Optimizations
- [ ] Profile test suite to identify other slow tests
- [ ] Consider mocking I/O operations in integration tests to enable synctest usage
- [ ] Evaluate if any production code with infinite loops can be refactored for better testability

### Documentation
- [ ] Create SYNCTEST_GUIDE.md with:
  - When to use synctest vs traditional testing
  - Examples of compatible vs incompatible test patterns
  - Migration guide for existing tests

## Notes

### Synctest Compatibility

**✅ Compatible with synctest:**
- `time.Sleep()` - becomes instant
- `time.After()` - instant fake time
- Channels created within the bubble
- `sync.WaitGroup` initialized within the bubble
- Goroutines spawned within the bubble

**❌ Incompatible with synctest:**
- System calls (`signal.Notify`, `os.FindProcess`)
- I/O operations (`os.Pipe`, `net.Pipe`, file/network I/O)
- External mocks/channels created outside the test
- Production code with infinite loops (testing schedulers/continuous processes)

### Key Learning
All resources (mocks, channels, goroutines) must be created INSIDE `synctest.Test()` function for durable blocking to work properly.
