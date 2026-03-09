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
- [ ] Run tests and code-generation inside Docker (instead of requiring Go to be installed locally)
- [ ] Feature Properties
- [ ] Features should be stored in repo by ID NOT key as keys can change leaving us with dangling keys
- [ ] Strategies should support arrays in the Context like the other SDKs
- [ ] Usage
- [ ] Check the percentage calc is the same in golang as everywhere else
- [ ] Can SSE server side eval be supported?
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
- If strategy has a percentage then hash on userkey or session and decide
- If the percentage doesn't match then continue with the next strategy
- If percentage matches (or there is no percentage) then continue and iterate through the attributes
	- If the attribute doesn't match then fall back and continue with the next strategy
	- If attribute matches then continue and check the next attribute
	- If all attributes match then we return the value from this strategy
	- Otherwise continue with the next strategy
- If no strategies match then return the default value for the feature

in SetRepository on Config, when a usage adapter is created, register a new plugin which has access to the current Config and when it receives an event, it checks if the Config's client has been created and if EdgeType() is PassiveRest and if so, calls Poll() on the client 