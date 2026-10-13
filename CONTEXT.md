# qdjr-admin

Admin backend and UI for qdjr.dev: posts, taxonomy, media, profile, settings, inbox, users, and the owner's self-study tracker.

## Language

### Learning

**Course**:
One external thing the owner is studying — a book, a video course, a documentation set. Has a title, url, provider and notes.
_Avoid_: track, path, curriculum

**Lesson**:
One unit of a Course, ordered by position. Has a title, url, status and notes.
_Avoid_: chapter, module, item

**Lesson status**:
`todo`, `in_progress` or `done`. Setting `done` stamps the completion time; leaving `done` clears it.
_Avoid_: state, progress (that word means the derived count below)

**Progress**:
A Course's `done / total` lesson count, derived on read and never stored.
