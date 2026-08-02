# Screenshots and recordings

Drop capture files here. Anything in `docs/public/` is served from the site root, so
`docs/public/shots/home.png` is `/shots/home.png` in a page.

## Attaching one to a page

Every content page already has a `<Shot>` slot with a stable `id` and an `alt` that
describes the screen. Add `src` for a still, or `video` for a recording:

```mdx
<Shot
  id="home"
  src="/shots/home.png"
  alt="The Home screen grouped into Review requested, Returned to you, …"
  caption="Nine merge requests need this person."
/>
```

```mdx
<Shot
  id="browse-tree"
  video="/shots/browse-tree.mp4"
  src="/shots/browse-tree.png"
  alt="Pressing b opens the group tree. Walking down platform ▸ ingest scopes …"
  caption="One key opens the tree. Enter on a group scopes the table beneath it."
/>
```

`video` plays muted, looped, and inline, with `src` as its poster frame. Nothing else on
the page moves when you attach a file: the frame already holds the right aspect ratio,
set by `cols` and `rows` (120×32 by default).

## Rules

- `alt` stays even after a file is attached. Until then it renders as visible prose, so
  a page with no capture still tells the reader what they would be looking at.
- Capture at **120×32** unless the page overrides `cols` and `rows`.
- Use the `ember` palette and a Nerd Font, so every screen on the site matches.
- Recordings: keep them under about 20 seconds, no cursor teleporting, one idea per clip.
  `vhs` `.tape` files belong next to the output so a capture can be regenerated.
- Never capture a real token, a real internal hostname, or a colleague's name.
