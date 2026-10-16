import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router";
import TasksPage from "./TasksPage";
import type { Task } from "../lib/api";

// Fixed clock so "overdue" is deterministic: today is 2026-10-15.
beforeEach(() => {
  vi.useFakeTimers({
    shouldAdvanceTime: true,
    now: new Date(2026, 9, 15, 12, 0, 0),
  });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function listResponse(tasks: Task[]) {
  return jsonResponse(200, { data: tasks });
}

function task(over: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "Buy milk",
    status: "open",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
    ...over,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/tasks"]}>
      <TasksPage />
    </MemoryRouter>,
  );
}

describe("TasksPage", () => {
  it("shows a skeleton while tasks load, then rows with badges", async () => {
    let resolveList!: (r: Response) => void;
    const fetchMock = vi.fn().mockImplementationOnce(
      () =>
        new Promise<Response>((res) => {
          resolveList = res;
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { container } = renderPage();
    expect(screen.getByRole("heading", { name: "Tasks" })).toBeDefined();
    expect(container.querySelector(".skeleton")).not.toBeNull();

    await act(async () => {
      resolveList(
        listResponse([
          task({ id: "t1", title: "Late", due_date: "2026-10-14" }),
          task({ id: "t2", title: "Late done", status: "done" }),
          task({ id: "t3", title: "Fine", due_date: "2026-10-20" }),
        ]),
      );
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Late" })).toBeDefined(),
    );
    expect(screen.getByText("overdue")).toBeDefined();
    expect(screen.getByText("done")).toBeDefined();
    expect(screen.getByText("open")).toBeDefined();
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/tasks");
  });

  it("filters by status and due day through query params", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([task()]))
      .mockResolvedValue(listResponse([task()]));
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Buy milk" })).toBeDefined(),
    );

    fireEvent.change(screen.getByLabelText("Status filter"), {
      target: { value: "done" },
    });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/tasks?status=done");

    fireEvent.change(screen.getByLabelText("Due date filter"), {
      target: { value: "2026-10-20" },
    });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(fetchMock.mock.calls[2]?.[0]).toBe(
      "/api/tasks?status=done&due=2026-10-20",
    );
  });

  it("completes a task with PATCH and reopens it again", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([task()]))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(
        listResponse([task({ status: "done", completed_at: "now" })]),
      )
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(listResponse([task()]));
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Complete Buy milk" }),
      ).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Complete Buy milk" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Reopen Buy milk" }),
      ).toBeDefined(),
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/tasks/t1");
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({ status: "done" }),
    });
    expect(fetchMock.mock.calls[2]?.[0]).toBe("/api/tasks");

    fireEvent.click(screen.getByRole("button", { name: "Reopen Buy milk" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Complete Buy milk" }),
      ).toBeDefined(),
    );
    expect(fetchMock.mock.calls[3]?.[0]).toBe("/api/tasks/t1");
    expect(fetchMock.mock.calls[3]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({ status: "open" }),
    });
  });

  it("renders an error and retries on failure", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("<html>boom</html>", { status: 200 }))
      .mockResolvedValueOnce(listResponse([task()]));
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("Couldn't load tasks.")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Buy milk" })).toBeDefined(),
    );
  });

  it("offers the create CTA when no tasks exist yet", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("No tasks yet.")).toBeDefined(),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Create your first task" }),
    );
    expect(screen.getByRole("heading", { name: "New task" })).toBeDefined();
  });

  it("says so when filters match nothing", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(listResponse([])));

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("No tasks yet.")).toBeDefined(),
    );

    fireEvent.change(screen.getByLabelText("Status filter"), {
      target: { value: "done" },
    });
    await waitFor(() =>
      expect(screen.getByText("No tasks match these filters.")).toBeDefined(),
    );
  });

  it("opens the edit form from a row and closes it on cancel", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValueOnce(listResponse([task()])),
    );

    renderPage();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Buy milk" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Buy milk" }));
    expect(screen.getByRole("heading", { name: "Edit task" })).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("heading", { name: "Edit task" })).toBeNull();
  });

  it("opens the create form from the header button", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    renderPage();
    await waitFor(() =>
      expect(screen.getByText("No tasks yet.")).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New task" }));
    expect(screen.getByRole("heading", { name: "New task" })).toBeDefined();
  });

  it("surfaces a failed status toggle as an error", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([task()]))
      .mockResolvedValueOnce(
        new Response("<html>boom</html>", { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Complete Buy milk" }),
      ).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Complete Buy milk" }));

    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("Couldn't update the task.")).toBeDefined();
  });

  it("ignores results that land after unmount", async () => {
    let resolveList!: (r: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((res) => {
            resolveList = res;
          }),
      ),
    );
    const first = renderPage();
    first.unmount();
    await act(async () => {
      resolveList(listResponse([task()]));
    });

    let rejectFetch!: (e: unknown) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((_, rej) => {
            rejectFetch = rej;
          }),
      ),
    );
    const second = renderPage();
    second.unmount();
    await act(async () => {
      rejectFetch(new TypeError("too late"));
    });
  });
});
