import type { CalendarDateProps, CalendarMonthProps } from "cally";
import type * as React from "react";

// Cally ships HTMLElementTagNameMap but no JSX types. Props are the
// library's own lists plus the React host attrs we actually use.
// Cally dispatches a non-bubbling change event that React never delivers,
// so `onchange` (lowercase, native listener) is the wiring that works.
declare module "react" {
  namespace JSX {
    interface IntrinsicElements {
      "calendar-date": CalendarDateProps & {
        className?: string;
        children?: React.ReactNode;
        ref?: React.Ref<HTMLElementTagNameMap["calendar-date"]>;
        onchange?: React.ChangeEventHandler<HTMLElement>;
      };
      "calendar-month": CalendarMonthProps & {
        className?: string;
        children?: React.ReactNode;
        ref?: React.Ref<HTMLElementTagNameMap["calendar-month"]>;
      };
    }
  }
}
