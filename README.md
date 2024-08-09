Overview
--------

My bootleg script to automate iTerm2 sessions.

Usage
-----

Setup:

* Enable Python API in iTerm2: https://iterm2.com/python-api-auth.html

Create a config file.

* This defines the iTerm2 sessions to start.
* Use `depends_on` to ensure a script from one session has completed before
  running scripts in another session.

```toml
# Single file config example

# Required - Unique id. Used to find/terminate the iterm window.
#   Use different ids if you wahave different projects
id = "my-project-local-dev"

# Working directory for all sessions.
# If unset, assumes the current working directory.
directory = "~/code/saas-control-plane/"

# Must create a section called `sessions`
[sessions]

# `sessions.<name>` defines an iterm session.
[sessions.setup]
# `script` - the script to run.
#   Use a script when you want to the tool to wait until the commands are all done.
script = '''
sleep 5
echo 'Setup is done'
'''

[sessions.server]
# `depends_on` - a list of dependent sessions.
#    This session will not start until these other sessions have completed.
#    Remember that the tool only waits for `script` scripts to complete.
depends_on = ["sessions.setup"]
# `inject` - inject commands into the session, as if you had typed them.
#    The tool executes these but does not wait for them to complete.
#    This is useful if you want to manually press up-arrow to re-run commands.
inject = '''
echo 'This is where the server would start'
'''

[sessions.worker]
depends_on = ["sessions.setup"]
inject = '''
echo 'This is where the worker would start'
'''
```

Run the tool. It will exit when all sessions have completed running their `script` and `inject`
scripts.

```
go run . -c example.toml
```

If you run the tool again, it will close the existing window, and create a new one and run all
scripts from the beginning. It matches windows based on the `id` in the config file.


Implementation
--------------

It uses a vendored and modified version of https://github.com/marwan-at-work/iterm2 to interact with the iTerm2 Python API from Golang (see `iterm2` directory)

Some cached state (window ids) is stored in `$HOME/.cache/itt-pglass-iterm-tool-cache`. The cache is how finds and terminates an existing iTerm2 window that was started by the tool.
