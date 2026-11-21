# 0011: `group activate` always takes a URL and creates the group it does not find

One call is the run-or-raise of a group, so a missing group is not a failure of its own. The call
still fails where it cannot tell which profile holds or gets the group: exit 2 with several
profiles and no window id, exit 4 where a host gives no reply.

Rejected: a `group activate <title>` without a URL that raises a group or fails, because a raise
that can fail on a missing group needs a "not found" exit code that nothing else needs.

Rejected: `tab open --group <title> <url>`, because `group activate` opens into a group when it
creates one and `group add` puts any tab into a group, so the flag is a second way to do the same.
