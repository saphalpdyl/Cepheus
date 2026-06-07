defmodule CepheusWeb.DashboardHTML do
  @moduledoc """
  Function components and presentation helpers for `CepheusWeb.DashboardLive`.

  Page-sized templates (`index_view`, `show_view`) live as `.heex` files in
  `dashboard_html/` and are compiled into functions on this module via
  `embed_templates/1`. Small reusable components (`toolbar`) and presentation
  helpers (`fmt_rtt`, `device_online?`, …) stay here.
  """
  use CepheusWeb, :html

  alias Cepheus.Dashboard

  embed_templates "dashboard_html/*"

  attr :window, :string, required: true
  attr :windows, :list, required: true
  attr :last_loaded_at, :any, required: true

  def toolbar(assigns) do
    ~H"""
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-xs text-base-content/60">
        Last refreshed {fmt_dt(@last_loaded_at)} · auto-refresh every 2s
      </p>
      <form phx-change="set_window" class="flex items-center gap-2">
        <label for="window" class="text-xs text-base-content/70">Window</label>
        <select id="window" name="window" class="select select-bordered select-sm">
          <option :for={w <- @windows} value={w} selected={w == @window}>{w}</option>
        </select>
      </form>
    </div>
    """
  end

  attr :event, :string, required: true
  attr :available, :list, required: true
  attr :selected, :any, required: true, doc: "nil (all) or list of selected target strings"
  attr :label, :string, default: "Targets"

  @doc """
  A horizontal chip-list of checkboxes for filtering visualizations by target.
  Submits the full selection on every change via `phx-change={@event}`. A `nil`
  selection means "all" and renders every box as checked.
  """
  def target_selector(assigns) do
    ~H"""
    <form phx-change={@event} class="flex flex-wrap items-center gap-x-3 gap-y-1">
      <span class="text-xs text-base-content/70">{@label}:</span>
      <span :if={@available == []} class="text-xs text-base-content/50">no targets yet</span>
      <label :for={t <- @available} class="flex cursor-pointer items-center gap-1">
        <input
          type="checkbox"
          name="targets[]"
          value={t}
          checked={target_checked?(@selected, t)}
          class="checkbox checkbox-xs"
        />
        <span class="font-mono text-xs">{t}</span>
      </label>
    </form>
    """
  end

  def target_checked?(nil, _t), do: true
  def target_checked?(selected, t) when is_list(selected), do: t in selected

  attr :rows, :list, required: true, doc: "queued task rows (form state)"
  attr :tasks, :list, required: true, doc: "the config's existing tasks (for recommendations)"

  @doc """
  A list-style editor for queueing one or more new probe tasks for an agent.
  Each row picks a task type + target/port/interval; submitting inserts them
  and bumps the agent generation. Traceroute rows without a same-IP latency
  probe surface a "recommended" ping/STAMP suggestion.
  """
  def task_form(assigns) do
    assigns =
      assign(assigns, :recommendations, Dashboard.recommended_trace_targets(assigns.rows, assigns.tasks))

    ~H"""
    <section class="card bg-base-100 border border-base-300">
      <div class="card-body gap-4">
        <div class="flex items-center justify-between gap-2">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-base-content/70">
            Add probe tasks
          </h2>
          <span :if={@rows != []} class="text-xs text-base-content/50">
            saving inserts the tasks &amp; bumps the agent generation
          </span>
        </div>

        <form id="add-tasks-form" phx-change="task_form_change" phx-submit="save_tasks" class="flex flex-col gap-2">
          <p :if={@rows == []} class="text-sm text-base-content/60">
            No tasks queued. Add one below to push new probes to this agent.
          </p>

          <div
            :for={row <- @rows}
            id={"task-#{row.ref}"}
            class="flex flex-wrap items-center gap-2 rounded-lg border border-base-300 bg-base-200/40 px-3 py-2"
          >
            <select name={"tasks[#{row.ref}][type]"} class="select select-bordered select-sm w-40">
              <option :for={{label, value} <- Dashboard.task_types()} value={value} selected={value == row.type}>
                {label}
              </option>
            </select>

            <span class="text-base-content/40">→</span>

            <input
              :if={Dashboard.task_has_target?(row.type)}
              type="text"
              name={"tasks[#{row.ref}][target]"}
              value={row.target}
              placeholder="target IP / host"
              phx-debounce="300"
              autocomplete="off"
              class="input input-bordered input-sm w-48 font-mono"
            />

            <label
              :if={Dashboard.task_has_port?(row.type)}
              class="flex items-center gap-1 text-xs text-base-content/60"
            >
              {Dashboard.port_label(row.type)}
              <input
                type="number"
                min="1"
                max="65535"
                name={"tasks[#{row.ref}][port]"}
                value={row.port}
                phx-debounce="300"
                class="input input-bordered input-sm w-24"
              />
            </label>

            <label
              :if={Dashboard.task_schedulable?(row.type)}
              class="flex items-center gap-1 text-xs text-base-content/60"
            >
              every
              <input
                type="number"
                min="1"
                name={"tasks[#{row.ref}][interval]"}
                value={row.interval}
                placeholder={Dashboard.default_interval(row.type)}
                phx-debounce="300"
                class="input input-bordered input-sm w-20"
              />s
            </label>
            <span :if={not Dashboard.task_schedulable?(row.type)} class="text-xs text-base-content/50">
              long-running listener
            </span>

            <button
              type="button"
              phx-click="remove_task_row"
              phx-value-ref={row.ref}
              class="btn btn-ghost btn-sm btn-square ml-auto text-base-content/50 hover:text-error"
              aria-label="Remove task"
            >
              <.icon name="hero-x-mark" class="size-4" />
            </button>
          </div>

          <div
            :for={target <- @recommendations}
            class="flex flex-wrap items-center gap-2 rounded-lg border border-dashed border-warning/40 bg-warning/5 px-3 py-2 text-sm"
          >
            <.icon name="hero-light-bulb" class="size-4 text-warning shrink-0" />
            <span>Traceroute to <span class="font-mono">{target}</span> has no latency probe.</span>
            <span class="badge badge-warning badge-sm">Recommended</span>
            <div class="ml-auto flex gap-1">
              <button
                type="button"
                class="btn btn-xs btn-outline btn-warning"
                phx-click="add_task_row"
                phx-value-type="ping"
                phx-value-target={target}
              >
                + Ping
              </button>
              <button
                type="button"
                class="btn btn-xs btn-outline btn-warning"
                phx-click="add_task_row"
                phx-value-type="stamp-sender"
                phx-value-target={target}
              >
                + STAMP
              </button>
            </div>
          </div>

          <div class="flex items-center justify-between pt-1">
            <button type="button" phx-click="add_task_row" class="btn btn-sm btn-ghost gap-1">
              <.icon name="hero-plus" class="size-4" /> Add task
            </button>
            <button type="submit" disabled={@rows == []} class="btn btn-sm btn-primary gap-1">
              <.icon name="hero-check" class="size-4" />
              {if @rows == [], do: "Save", else: "Save #{length(@rows)} task(s)"}
            </button>
          </div>
        </form>
      </div>
    </section>
    """
  end

  def short_uuid(nil), do: ""
  def short_uuid(uuid) when is_binary(uuid), do: String.slice(uuid, 0, 8) <> "…"

  def fmt_dt(%DateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S UTC")
  def fmt_dt(%NaiveDateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")
  def fmt_dt(nil), do: "—"
  def fmt_dt(other), do: to_string(other)

  def fmt_loss(nil), do: "—"

  def fmt_loss(f) when is_float(f),
    do: :io_lib.format("~.2f", [f * 100]) |> IO.iodata_to_binary() |> Kernel.<>("%")

  def fmt_loss(other), do: to_string(other)

  def fmt_rtt(nil), do: "—"

  def fmt_rtt(ns) when is_integer(ns) do
    ms = ns / 1_000_000
    :io_lib.format("~.2f ms", [ms]) |> IO.iodata_to_binary()
  end

  def fmt_rtt(other), do: to_string(other)

  @doc """
  A device is considered "online" if its most recent measurement across all
  targets is within 2× its report interval.
  """
  def device_online?(device, summary) do
    threshold_seconds = max(device.report_interval_seconds * 2, 30)

    case summary |> Enum.map(& &1.last_seen) |> Enum.filter(& &1) do
      [] ->
        false

      stamps ->
        latest = Enum.reduce(stamps, &latest_dt/2)
        DateTime.diff(DateTime.utc_now(), latest) <= threshold_seconds
    end
  end

  defp latest_dt(a, b), do: if(DateTime.compare(a, b) == :gt, do: a, else: b)

  def summary_avg_p95([]), do: "—"

  def summary_avg_p95(summary) do
    case Enum.filter(summary, & &1.avg_rtt_p95_ns) do
      [] -> "—"
      rows -> rows |> Enum.map(& &1.avg_rtt_p95_ns) |> Enum.sum() |> div(length(rows)) |> fmt_rtt()
    end
  end

  def summary_worst_loss([]), do: "—"

  def summary_worst_loss(summary) do
    case Enum.filter(summary, & &1.loss) do
      [] -> "—"
      rows -> rows |> Enum.map(& &1.loss) |> Enum.max() |> fmt_loss()
    end
  end

  def fmt_severity(nil), do: "—"

  def fmt_severity(s) when is_float(s),
    do: :io_lib.format("~.2f", [s]) |> IO.iodata_to_binary()

  def fmt_severity(other), do: to_string(other)

  @doc """
  Maps an argus severity (0.0–1.0+) to a daisyUI badge variant.
  """
  def severity_badge_class(s) when is_float(s) and s >= 0.75, do: "badge-error"
  def severity_badge_class(s) when is_float(s) and s >= 0.5, do: "badge-warning"
  def severity_badge_class(s) when is_float(s) and s >= 0.25, do: "badge-info"
  def severity_badge_class(_), do: "badge-ghost"

  def event_status_badge_class("OPEN"), do: "badge-error"
  def event_status_badge_class("RECOVERING"), do: "badge-warning"
  def event_status_badge_class("RESOLVED"), do: "badge-success"
  def event_status_badge_class("open"), do: "badge-error"
  def event_status_badge_class("closed"), do: "badge-success"
  def event_status_badge_class(_), do: "badge-ghost"

  @doc "Truncate JSONB-as-text for inline display."
  def trunc_details(nil), do: ""
  def trunc_details(s) when is_binary(s) and byte_size(s) > 80, do: binary_part(s, 0, 80) <> "…"
  def trunc_details(s) when is_binary(s), do: s
end
