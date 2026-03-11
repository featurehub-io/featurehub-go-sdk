Todo
----
- [X] Config
- [X] Client interface
- [X] StreamingClient
- [X] Unit tests
- [X] Handle feature_delete events
- [X] Compare versions when "feature" event is received (don't just overwrite)
- [X] Allow notify / callback functions (add and remove)
- [X] Global "readyness" callback (either OK when data has arrived, or an error if there was a fail)
- [X] Removed support for server-side ClientContext, and submit this as an x-featurehub header upon connection
- [X] Dockerfile for building and running the todo-server
- { ] e2e tests using the javascript client image once Alex figures it out
- [X] Feature Properties
- [ ] Polling should be able to handle multi SDK Keys
- [ ] Can SSE server side eval be supported?
- [X] Strategies should support arrays in the Context like the other SDKs
- [X] Feature updates should deal with key changes, ID is the unique identifier internally
- [X] Usage
- [X] Usage should include the environment id
- [ ] Usage otel
- [ ] Usage segment
- [X] Check the percentage calc is the same in golang as everywhere else
- [X] Client-side rollout strategies (https://github.com/featurehub-io/featurehub/tree/master/backend/sse-strategy-matchers/src)
	- [x] Percentages [==, !=]
	- [x] Country [==, !=]
	- [x] Device [==, !=]
	- [x] Platform [==, !=]
	- [x] Version [==, !=, >, >=, <, <=]
	- [x] Custom
		- [x] string [==, !=, startsWith, endsWith, <, <=, >, >=, excludes, includes, regex]
		- [x] semver [==, !=, startsWith, endsWith, <, <=, >, >=, excludes, includes, regex]
		- [x] number [==, !=, <, <=, >, >=, excludes, includes]
		- [x] date [==, !=, startsWith, endsWith, <, <=, >, >=, excludes, includes, regex]
		- [x] date-time [==, !=, startsWith, endsWith, <, <=, >, >=, excludes, includes, regex]
		- [x] boolean [==, !=]
		- [x] ip-address [==, !=, excludes, includes]

Strategy matching logic:
- If strategy has a percentage then figure out the percentage text. If PercentageAttributes is empty or nil, 
it will be SessionId or UserKey. If neither exists, SessionId will be randomised and allocated into the context for the
session. If PercentageAttributes is set, it will collect each attribute from the context and concatenate them with a `$`
separator. The final percentage text to evaluate against is this text + featureID. Features
can have strategies that are mixes of different percentages with different PercentAttributes, so we keep track of where we are
with each key and accumulate the totals as we go. Percentages are not absolute, they are relative to the last percentage _using that key_,
so 33% followed by 40% will cover 0>= <=33%, >33%, <= 73%. If the 33% and 40% use different keys, they they will be evaluated as <= 33% and <= 40% on their own terms. 
If matched then attributes are compared if any.
 
- If the percentage doesn't match then continue with the next strategy
- If percentage matches (or there is no percentage) then continue and iterate through the attributes
	- If the attribute doesn't match then fall back and continue with the next strategy
	- If attribute matches then continue and check the next attribute
	- If all attributes match then we return the value from this strategy
	- Otherwise continue with the next strategy
- If no strategies match then return the default value for the feature
