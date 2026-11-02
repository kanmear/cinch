# Scan the SMS inbox less often in the background

The app reads the banking SMS inbox in the background, when it starts and when it comes back to the foreground. Right now such a background scan is allowed at most once a minute. That is far too frequent and wastes battery.

Change it so that background scans run at most once every 13 minutes.

- A background scan that comes less than 13 minutes after the previous scan finished is skipped. Once 13 minutes have passed, the next one runs.
- Everything else about background scans stays as it is: they still need the read permission to be granted already and an existing scan point, they never prompt, and two scans never run at the same time. A scan that fails still counts as a scan for this purpose.

The interface is the existing one: `SmsScanCubit.scanInBackground()` on a cubit built as today, with the injected `Clock`. Nothing in the constructor changes.

Follow the repository's conventions. Add or update tests. Commit your work on the current branch when done.
