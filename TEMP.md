## Questions

- How will we pass information about the datasource to the mimirrule_controller?
  - A: Through `osko.dev/datasourceRef: "logging-ds"`
- Running locally:
  - The Mimirrule object is still created as non-ready, though. It would be pretty
    cool if the finalizer wasn't registered until the object is succesfully created
    in Mimir.
    - A: We will probably move the finalizer to the `createMimirRuleGroupAPI` method.
- Am I correct to think that most (if not all) functions in
  `internal/controller/osko/mimirrule_controller.go` are either "just" wrapping MimirClient
  or are unused?
  - A: YES

## General remarks

- We probably shouldn't be using `billing` tenant for testing, even on dev...
  Couldn't find anything that would be broken because of this, but still seems kinda
  troll.
  - A: W/e, if causes issue we will figure it out
- I somewhat dislike that we use both longform error handling with `if err != nil`, as well as
  `; err != nil` - unless of course I am missing some functionality there.
  - A: I am just not great enough yet

## Stuff to fix/add

- Check if mimirrule still exists in the Mimir API if we have mimirrule CRD present (periodical API check against Mimir)
- If source_tenant is added, MimirRule doesn't get reconciled/changed with that new source_tenant
- Make sure we handle Datasource missing referenced by SLO `osko.dev/datasourceRef` annotation
