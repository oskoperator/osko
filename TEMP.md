## Questions

- How will we pass information about the datasource to the mimirrule_controller?
- Running locally:
  - Interestingly to me, if I am running the operator locally (`make run`), there are
    no rules created in Mimir, even when I am on the VPN. Ideas why?
  - The Mimirrule object is still created as non-ready, though. It would be pretty
    cool if the finalizer wasn't registered until the object is succesfully created
    in Mimir.
  - This would probably mean somehow calling `mimirRule.GetFinalizers()` during
    `helpers.NewMimirRule`, which has seems would have its own challenges
- Am I correct to think that most (if not all) functions in
  `internal/controller/osko/mimirrule_controller.go` are either "just" wrapping MimirClient
  or are unused?

## General remarks

- We probably shouldn't be using `billing` tenant for testing, even on dev...
  Couldn't find anything that would be broken because of this, but still seems kinda
  troll.
- I somewhat dislike that we use both longform error handling with `if err != nil`, as well as
  `; err != nil` - unless of course I am missing some functionality there.
