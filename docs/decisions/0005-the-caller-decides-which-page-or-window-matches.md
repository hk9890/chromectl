# 0005: chromectl prints a tab's title and URL as Chrome gives them, and the caller decides which page or window matches

What counts as the same page (a trailing slash, a query, a fragment, a redirect) is a policy, and
chromectl holds none. The one place chromectl reads a URL to choose is inside a group its title has
already found, where `group activate` ranks the group's tabs by the URL it was given. There the URL
never decides that a page is open.

Rejected: one call that finds a tab by a URL prefix or opens the URL, because it makes chromectl
the judge of when a page is open.

Rejected: printing the window manager's title, `<tab title> - Google Chrome`, because no extension
can read the suffix, which differs per browser, and a window the user named shows the name alone.
