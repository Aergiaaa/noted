import { useEffect, useMemo, useRef, type MouseEvent } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";
import { useNavigate } from "react-router";
import {
  findMentions,
  mentionRoute,
  resolveMentions,
} from "../../lib/mentions";
import { createLinkTitleMap } from "../../lib/selectors";
import { useLinkSources } from "../links/useLinkSources";

interface NoteBodyProps {
  body: string;
  className?: string;
}

// marked parses the markdown, DOMPurify sanitizes the resulting HTML —
// SECURITY.md allows only sanitized HTML into the page (no raw injection).
// Mention tokens survive that pipeline as plain text; the effect below
// swaps them for chips built as live DOM nodes AFTER sanitize (F9: never
// string-concatenated HTML). React owns the innerHTML (dangerouslySetInnerHTML)
// so a body change resets it, while the chip swap persists across renders
// until then — the walker is idempotent, so a re-run never double-inserts.
export default function NoteBody({ body, className }: NoteBodyProps) {
  const html = useMemo(
    () => DOMPurify.sanitize(marked.parse(body, { async: false })),
    [body],
  );

  const mentionKinds = useMemo(
    () => [...new Set(findMentions(body).map((token) => token.type))],
    [body],
  );

  const sources = useLinkSources(mentionKinds);
  const titles = useMemo(() => createLinkTitleMap(sources), [sources]);
  const containerRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    if (sources.loading || mentionKinds.length === 0) return;

    const walker = document.createTreeWalker(
      containerRef.current as HTMLDivElement,
      NodeFilter.SHOW_TEXT,
    );
    const textNodes: Text[] = [];

    while (walker.nextNode()) {
      textNodes.push(walker.currentNode as Text);
    }

    for (const node of textNodes) {
      const segments = resolveMentions(node.data, titles);

      if (segments.length === 1 && segments[0].kind === "text") continue;

      const fragment = document.createDocumentFragment();

      for (const segment of segments) {
        if (segment.kind === "text") {
          fragment.append(segment.text);
          continue;
        }

        if (segment.title !== null) {
          const anchor = document.createElement("a");
          anchor.href = mentionRoute(segment.type, segment.id);
          anchor.className = "badge badge-info badge-sm";
          anchor.dataset.mention = "true";
          anchor.textContent = segment.title;
          fragment.append(anchor);
        } else {
          const muted = document.createElement("span");
          muted.className = "text-base-content/40";
          muted.textContent = segment.raw;
          fragment.append(muted);
        }
      }

      node.replaceWith(fragment);
    }
  }, [sources.loading, mentionKinds, titles, html]);

  // Chips are plain anchors in the sanitized tree (not React children), so
  // navigation is delegated here — SPA-style, never a full page load.
  const onClick = (event: MouseEvent<HTMLDivElement>) => {
    const anchor = (event.target as HTMLElement).closest("a[data-mention]");
    if (anchor === null) return;
    event.preventDefault();
    navigate(anchor.getAttribute("href") as string);
  };

  return (
    <div
      ref={containerRef}
      onClick={onClick}
      className={["text-sm", className].filter(Boolean).join(" ")}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
