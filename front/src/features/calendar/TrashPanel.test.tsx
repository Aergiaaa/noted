import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import TrashPanel from "./TrashPanel";
import type { Event } from "../../lib/api";
import { eventColorDotClasses } from "./eventColor";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const deleted: Event = {
  id: "e9",
  title: "Dentist",
  starts_at: "2026-10-14T09:00:00.000Z",
  ends_at: "2026-10-14T10:00:00.000Z",
};

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function listResponse(events: Event[]) {
  return jsonResponse(200, { data: events });
}

function panel(
  overrides: Partial<Parameters<typeof TrashPanel>[0]> = {},
  fetchMock = vi.fn().mockResolvedValue(listResponse([deleted])),
) {
  vi.stubGlobal("fetch", fetchMock);
  return render(
    <TrashPanel
      onClose={vi.fn()}
      onRestored={vi.fn()}
      onRestoreError={vi.fn()}
      {...overrides}
    />,
  );
}

async function waitForListedDentist() {
  await waitFor(() => expect(screen.getByText("Dentist")).toBeDefined());
}

describe("TrashPanel", () => {
  it("lists deleted events and restores on click", async () => {
    const onRestored = vi.fn();
    let listCalls = 0;
    const fetchMock = vi
      .fn()
      .mockImplementation((_path: string, init?: RequestInit) =>
        init?.method === "POST"
          ? Promise.resolve(jsonResponse(200, deleted))
          : (listCalls += 1) === 1
            ? Promise.resolve(listResponse([deleted]))
            : Promise.resolve(listResponse([])),
      );
    panel({ onRestored }, fetchMock);

    await waitForListedDentist();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));

    await waitFor(() =>
      expect(screen.getByText("Nothing in the trash.")).toBeDefined(),
    );
    expect(onRestored).toHaveBeenCalledOnce();
    const restoreCall = fetchMock.mock.calls.find(
      ([, init]) => (init as RequestInit | undefined)?.method === "POST",
    );
    expect(restoreCall?.[0]).toBe("/api/events/e9/restore");
  });

  it("marks each row with the event's hashed color dot", async () => {
    panel();

    await waitForListedDentist();
    expect(screen.getByTestId("event-color-dot").className).toContain(
      eventColorDotClasses("e9"),
    );
  });

  it("surfaces a server restore error through onRestoreError", async () => {
    const onRestoreError = vi.fn();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([deleted]))
      .mockResolvedValueOnce(
        jsonResponse(500, {
          error: { code: "internal_error", message: "boom" },
        }),
      );
    panel({ onRestoreError }, fetchMock);

    await waitForListedDentist();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));

    await waitFor(() => expect(onRestoreError).toHaveBeenCalledWith("boom"));
  });

  it("falls back to the generic message when the restore response is not JSON", async () => {
    const onRestoreError = vi.fn();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([deleted]))
      .mockResolvedValueOnce(new Response("nope", { status: 200 }));
    panel({ onRestoreError }, fetchMock);

    await waitForListedDentist();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));

    await waitFor(() =>
      expect(onRestoreError).toHaveBeenCalledWith(
        "Couldn't restore the event.",
      ),
    );
  });

  it("shows skeletons while loading", async () => {
    let resolveList!: (r: Response) => void;
    const { container } = panel(
      {},
      vi.fn().mockImplementation(
        () =>
          new Promise<Response>((res) => {
            resolveList = res;
          }),
      ),
    );

    expect(container.querySelectorAll(".skeleton")).toHaveLength(3);
    expect(screen.queryByText("Dentist")).toBeNull();

    await act(async () => {
      resolveList(listResponse([deleted]));
    });
    await waitForListedDentist();
  });

  it("shows the error state with retry", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(500, {
          error: { code: "internal_error", message: "boom" },
        }),
      )
      .mockResolvedValueOnce(listResponse([]));
    panel({}, fetchMock);

    await waitFor(() => expect(screen.getByText("boom")).toBeDefined());

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getByText("Nothing in the trash.")).toBeDefined(),
    );
  });

  it("falls back to the generic load message when the list body is not JSON", async () => {
    panel(
      {},
      vi
        .fn()
        .mockResolvedValue(new Response("<html>nope</html>", { status: 200 })),
    );

    await waitFor(() =>
      expect(screen.getByText("Couldn't load events.")).toBeDefined(),
    );
  });

  it("shows the empty state when nothing was deleted", async () => {
    panel({}, vi.fn().mockResolvedValue(listResponse([])));
    await waitFor(() =>
      expect(screen.getByText("Nothing in the trash.")).toBeDefined(),
    );
  });

  it("closes on backdrop click but not on inner clicks", async () => {
    const onClose = vi.fn();
    const { container } = panel({ onClose });

    await waitForListedDentist();
    fireEvent.click(screen.getByText("Dentist"));
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(container.querySelector(".modal") as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
