# 0008: The extension's permission set is closed: a command that needs a new permission is a different product

The extension asks for what the job needs and for nothing that lets the host read a page.
chromectl is not a browser automation tool: it does not drive pages, fill forms or wait for loads.

Rejected: reading page content, running scripts and screenshots, because `<all_urls>` and
`scripting` are a different trust level. That work belongs to the DevTools protocol and a test
profile, not to the user's daily browser.
