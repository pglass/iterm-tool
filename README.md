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

# `id` is required. It is a unique id for this project.
#
#   The tool allows at most one iterm window running per id at a time.
#   When it launches a window, it terminates any existing window with
#   the same id and starts a new window from scratch.
#
id = "my-project-local-dev"

# `directory` is the working directory for all sessions in this file.
#
#   If unset, assumes the current working directory.
#   (This example uses '.' - the current directory - so that the example
#   is runnable on any machine.)
directory = "."

# `sessions.<name>` defines an iterm session.
#
#    All configured sessions must have the `sessions.` prefix.
#
[sessions.setup]
# `script` - A script to run in the session.
#
#    The tool waits for the script to complete.
#    Try to avoid using a `script` for processes that never complete (instead, use `inject`).
#    If both `script` and `inject` are specifed, the tool starts the `script` and waits for
#    it to complete before running the `inject` commands.
#
script = '''
sleep 5
echo 'Setup is done'
'''

[sessions.server]
# `depends_on` - a list of dependent sessions that must start/complete first.
#
#    This session will not start until these other sessions have completed.
#    The tool only waits for `script` blocks to complete. It does not wait for `inject` blocks.
#    If you add dependencies that have `script` blocks that never complete, then
#    this session will be never be able to start.
#
depends_on = ["sessions.setup"]
# `inject` - run commands into the session, as if you had typed them.
#
#    The tool executes these but does not wait for them to complete. Use
#    `inject` if you do not want the tool to wait on processes, such as processes
#    that will never complete. If both `script` and `inject` are specified, the
#    `script` is run first and then `inject` is run after `script` completes.
#
#    `inject` commands are run in the session as if they were typed. This means
#    you can use `inject` to build up terminal history and set environment variables.
#    Then
#
inject = '''
echo 'This is where the server would start'
'''

# Session naming and grouping
#
#     - `session.<name>` defines a named sessions
#     - `session.<group>.<name>` defines a grouped session
#
# Grouping implies a certain layout in the iterm2 window.
# Each group is assigned a column (vertical split pane).
# Each member of a group is horizontally split its group column.
[sessions.worker.1]
depends_on = ["sessions.setup"]
inject = '''
echo 'This is where the worker 1 would start'
'''

[sessions.worker.2]
depends_on = ["sessions.setup"]
inject = '''
echo 'This is where the worker 2 would start'
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
