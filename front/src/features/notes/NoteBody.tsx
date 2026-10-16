import { useMemo } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";

interface NoteBodyProps {
  body: string;
  className?: string;
}

// marked parses the markdown, DOMPurify sanitizes the resulting HTML —
// SECURITY.md allows only sanitized HTML into the page (no raw injection).
export default function NoteBody({ body, className }: NoteBodyProps) {
  const html = useMemo(
    () => DOMPurify.sanitize(marked.parse(body, { async: false })),
    [body],
  );
  return (
    <div
      className={["text-sm", className].filter(Boolean).join(" ")}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
