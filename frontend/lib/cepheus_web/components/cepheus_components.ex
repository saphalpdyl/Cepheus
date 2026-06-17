defmodule CepheusWeb.CepheusComponents do
  @moduledoc """
  Shared UI primitives for the Cepheus Tier-1 detection console.

  A 1:1 port of the React reference (`core_components.jsx` / `charts.jsx`) into
  plain Phoenix function components, plus the presentation helpers the pages
  need. Everything here is deliberately small and explicit — a human Elixir dev
  should be able to read a component top-to-bottom and edit it without chasing
  macros.

  Imported into every template via `CepheusWeb.html_helpers/0`. We `use
  Phoenix.Component` directly (rather than `use CepheusWeb, :html`) to avoid an
  import cycle, since this module is itself imported by `html_helpers`.
  """
  use Phoenix.Component

  use Phoenix.VerifiedRoutes,
    endpoint: CepheusWeb.Endpoint,
    router: CepheusWeb.Router,
    statics: CepheusWeb.static_paths()

  import CepheusWeb.CoreComponents, only: [icon: 1]
  alias Cepheus.Dashboard

  # ===========================================================================
  # Chrome
  # ===========================================================================

  @doc """
  A bordered card with an optional head (title + meta + actions) and a body.
  `body_class` mirrors the reference: `""` (default padding), `"tight"` (no
  padding, for full-bleed tables) or `"pad"` (roomier padding).
  """
  attr :title, :string, default: nil
  attr :meta, :string, default: nil
  attr :class, :string, default: nil
  attr :body_class, :string, default: ""
  slot :actions
  slot :inner_block, required: true

  def card(assigns) do
    ~H"""
    <section class={["rounded-lg border border-sl-200 bg-paper", @class]}>
      <div
        :if={@title || @meta || @actions != []}
        class="flex items-center gap-2 border-b border-sl-200 px-3 py-2"
      >
        <h3 :if={@title} class="text-sm font-semibold text-ink">{@title}</h3>
        <span :if={@meta} class="ml-auto text-xs text-sl-500">{@meta}</span>
        <div :if={@actions != []} class={[@meta == nil && "ml-auto"]}>{render_slot(@actions)}</div>
      </div>
      <div class={card_body_class(@body_class)}>{render_slot(@inner_block)}</div>
    </section>
    """
  end

  defp card_body_class("tight"), do: "p-0"
  defp card_body_class("pad"), do: "px-4 py-3"
  defp card_body_class(_), do: "p-3"

  @doc "A label-above / value-below key/value pair (used in the triage panel)."
  attr :k, :string, required: true
  attr :mono, :boolean, default: false
  slot :inner_block, required: true

  def kv(assigns) do
    ~H"""
    <div class="mb-2 flex flex-col gap-px">
      <span class="text-xs text-sl-500">{@k}</span>
      <span class={["text-ink", @mono && "font-mono text-xs tabular-nums" || "text-sm"]}>
        {render_slot(@inner_block)}
      </span>
    </div>
    """
  end

  @doc "Booktabs table header cell."
  attr :num, :boolean, default: false
  attr :class, :string, default: nil
  attr :rest, :global
  slot :inner_block

  def th(assigns) do
    ~H"""
    <th
      class={[
        "whitespace-nowrap border-b border-sl-200 px-3 py-2 text-xs font-medium text-sl-500",
        @num && "text-right" || "text-left",
        @class
      ]}
      {@rest}
    >
      {render_slot(@inner_block)}
    </th>
    """
  end

  @doc "Booktabs table body cell. The row owns the bottom border."
  attr :num, :boolean, default: false
  attr :mono, :boolean, default: false
  attr :muted, :boolean, default: false
  attr :class, :string, default: nil
  attr :rest, :global
  slot :inner_block

  def td(assigns) do
    ~H"""
    <td
      class={[
        "px-3 py-2 align-middle",
        @num && "text-right tabular-nums",
        @mono && "font-mono tabular-nums",
        @muted && "text-sl-400" || "text-sl-700",
        @class
      ]}
      {@rest}
    >
      {render_slot(@inner_block)}
    </td>
    """
  end

  @doc "Segmented time-window switch. Buttons emit `set_window`."
  attr :window, :string, required: true
  attr :windows, :list, required: true

  def window_switch(assigns) do
    ~H"""
    <div class="inline-flex gap-0.5 rounded-md border border-sl-200 bg-canvas-alt p-0.5" role="group">
      <button
        :for={w <- @windows}
        type="button"
        phx-click="set_window"
        phx-value-window={w}
        class={[
          "rounded-[3px] px-2 py-0.5 font-mono text-[11px] font-medium transition-colors",
          w == @window && "bg-paper text-ink shadow-sm" || "text-sl-500 hover:text-ink"
        ]}
      >
        {w}
      </button>
    </div>
    """
  end

  @doc "The Cepheus brand mark."
  attr :size, :integer, default: 24

  def cepheus_mark(assigns) do
    ~H"""
    <img src={~p"/images/logo.png"} width={@size} height={@size} alt="Cepheus" class="shrink-0" />
    """
  end

  # ===========================================================================
  # Pills, badges, metric blocks
  # ===========================================================================

  @doc "A tinted status pill. tone: ok | warn | crit | neutral | info."
  attr :tone, :string, default: "neutral"
  attr :dot, :boolean, default: false
  attr :class, :string, default: nil
  slot :inner_block, required: true

  def pill(assigns) do
    ~H"""
    <span class={[
      "inline-flex w-fit items-center gap-1.5 whitespace-nowrap rounded-[5px] px-2 py-0.5 font-mono text-xs font-medium tabular-nums",
      pill_class(@tone),
      @class
    ]}>
      <span :if={@dot} class="size-1.5 shrink-0 rounded-full bg-current"></span>
      {render_slot(@inner_block)}
    </span>
    """
  end

  defp pill_class("ok"), do: "bg-seu-50 text-seu-700"
  defp pill_class("warn"), do: "bg-unlearn-50 text-unlearn-700"
  defp pill_class("crit"), do: "bg-leak-50 text-leak-700"
  defp pill_class("info"), do: "bg-encoder-50 text-encoder-700"
  defp pill_class(_), do: "bg-canvas-alt text-sl-600"

  @doc "A metric block: small label above, pill value below."
  attr :label, :string, required: true
  attr :tone, :string, default: "neutral"
  attr :row, :boolean, default: false, doc: "lay label and value on one line"
  slot :inner_block, required: true

  def metric(assigns) do
    ~H"""
    <div class={["flex gap-1.5", @row && "flex-row items-baseline justify-between" || "flex-col"]}>
      <span class="text-xs text-sl-500">{@label}</span>
      <.pill tone={@tone}>{render_slot(@inner_block)}</.pill>
    </div>
    """
  end

  @doc "Derived agent health pill (nominal | degraded | alert)."
  attr :status, :string, required: true

  def status_pill(assigns) do
    ~H"""
    <.pill tone={status_tone(@status)} dot>{status_label(@status)}</.pill>
    """
  end

  defp status_tone("nominal"), do: "ok"
  defp status_tone("degraded"), do: "warn"
  defp status_tone("alert"), do: "crit"
  defp status_tone(_), do: "neutral"

  defp status_label("nominal"), do: "Healthy"
  defp status_label("degraded"), do: "Degraded"
  defp status_label("alert"), do: "Alert"
  defp status_label(other), do: to_string(other)

  @doc "Event status badge — expects a display status like \"OPEN\" / \"RESOLVED\"."
  attr :status, :string, required: true

  def event_badge(assigns) do
    ~H"""
    <span class={[
      "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide",
      event_badge_class(@status)
    ]}>
      <span class="size-[5px] rounded-full bg-current"></span>{String.downcase(@status)}
    </span>
    """
  end

  defp event_badge_class("OPEN"), do: "bg-leak-100 text-leak-700"
  defp event_badge_class("RECOVERING"), do: "bg-unlearn-100 text-unlearn-700"
  defp event_badge_class("RESOLVED"), do: "bg-seu-100 text-seu-700"
  defp event_badge_class(_), do: "bg-sl-100 text-sl-500"

  @doc "A severity number, toned by magnitude."
  attr :z, :any, required: true

  def sev(assigns) do
    assigns = assign(assigns, :tone, severity_tone(assigns.z))

    ~H"""
    <span class={[
      "inline-flex min-w-[40px] items-center justify-center rounded-[3px] px-1.5 py-0.5 font-mono text-xs font-semibold tabular-nums",
      sev_class(@tone)
    ]}>
      {fmt_severity(@z)}
    </span>
    """
  end

  defp sev_class("nominal"), do: "bg-sl-100 text-sl-500"
  defp sev_class("watch"), do: "bg-unlearn-50 text-unlearn-700"
  defp sev_class("warn"), do: "bg-leak-50 text-leak-700"
  defp sev_class("crit"), do: "bg-leak-600 text-white"

  @doc "A metric token chip (mono) with a human-readable tooltip via `title`."
  attr :metric, :string, required: true

  def metric_chip(assigns) do
    ~H"""
    <span
      class="inline-block rounded-[3px] border border-sl-200 bg-canvas-alt px-1.5 py-px font-mono text-xs text-sl-600"
      title={"family: #{Dashboard.metric_family(@metric)}"}
    >
      {@metric}
    </span>
    """
  end

  @doc "Detector-provenance badge (EWMA / BETA-BINOMIAL / …)."
  slot :inner_block, required: true

  def detector_badge(assigns) do
    ~H"""
    <span class="inline-flex items-center rounded-[3px] border border-encoder-600/20 bg-encoder-50 px-1.5 py-px font-mono text-[9.5px] uppercase tracking-wide text-encoder-700">
      {render_slot(@inner_block)}
    </span>
    """
  end

  # ===========================================================================
  # Detection primitives
  # ===========================================================================

  @doc """
  β-binomial loss cell: posterior mean % with a 95% credible-interval bar.
  `model` is `%{mean_pct, lo_pct, hi_pct}` (from `Dashboard.loss_posterior/2`) or
  `nil`. `cap` optionally fixes the bar's full-scale % so several cells share one
  scale.
  """
  attr :model, :any, required: true
  attr :cap, :any, default: nil

  def loss_cell(%{model: nil} = assigns) do
    ~H"""
    <span class="font-mono text-xs text-sl-400">—</span>
    """
  end

  def loss_cell(assigns) do
    m = assigns.model
    top = assigns[:cap] || max(0.6, m.hi_pct * 1.25)
    pc = fn v -> "#{Float.round(min(max(v / top * 100, 0.0), 100.0), 2)}%" end

    assigns =
      assigns
      |> assign(:band_left, pc.(m.lo_pct))
      |> assign(:band_width, pc.(m.hi_pct - m.lo_pct))
      |> assign(:mean_left, pc.(m.mean_pct))
      |> assign(:tone, loss_tone(m.mean_pct))

    ~H"""
    <div class="flex items-center gap-2">
      <span class={["min-w-[48px] text-right font-mono text-xs", loss_num_class(@tone)]}>
        {fmt_pct(@model.mean_pct)}
      </span>
      <div class="relative h-2 min-w-[80px] max-w-[150px] flex-1 overflow-hidden rounded-full border border-sl-200 bg-canvas-alt">
        <span
          class="absolute top-0 bottom-0 bg-unlearn-600/25"
          style={"left:#{@band_left};width:#{@band_width}"}
        >
        </span>
        <span class="absolute -top-px -bottom-px w-0.5 bg-unlearn-700" style={"left:#{@mean_left}"}>
        </span>
      </div>
      <span class="whitespace-nowrap font-mono text-[10px] text-sl-400">
        [{fmt_num(@model.lo_pct)}–{fmt_num(@model.hi_pct)}]
      </span>
    </div>
    """
  end

  defp loss_tone(mean) when mean > 0.5, do: "warn"
  defp loss_tone(mean) when mean > 0.05, do: "watch"
  defp loss_tone(_), do: "ok"

  defp loss_num_class("warn"), do: "font-semibold text-leak-700"
  defp loss_num_class("watch"), do: "text-unlearn-700"
  defp loss_num_class(_), do: "text-sl-600"

  @doc "A detection-family summary tile (fleet overview). Links to the alerts inbox."
  attr :family, :map, required: true

  def family_tile(assigns) do
    ~H"""
    <.link
      navigate={~p"/alerts"}
      class="flex flex-col items-start gap-2 border-r border-sl-200 p-3 px-4 text-left last:border-r-0 hover:bg-canvas-alt"
    >
      <span class="text-sl-500"><.icon name={family_icon(@family.id)} class="size-4" /></span>
      <span class="text-sm font-semibold text-ink">{@family.label}</span>
      <.detector_badge>{@family.detector}</.detector_badge>
      <span class="mt-0.5 flex flex-col gap-1">
        <.pill tone={(@family.open > 0 && "crit") || "ok"} dot>
          {(@family.open > 0 && "#{@family.open} open") || "Clear"}
        </.pill>
        <span class="font-mono text-[10px] text-sl-500">
          {@family.agents} agents · {@family.findings} findings
        </span>
      </span>
    </.link>
    """
  end

  defp family_icon("stamp"), do: "hero-bolt"
  defp family_icon("ping"), do: "hero-signal"
  defp family_icon("loss"), do: "hero-arrow-trending-down"
  defp family_icon(_), do: "hero-cube"

  @doc """
  The signature latency tick-strip: a resampled severity bar-chart. `values` is
  any numeric series (RTT ns, severity, …); bars are toned relative to the
  series' own max. `mini` renders a compact, chrome-less variant for table cells.
  """
  attr :values, :list, required: true
  attr :count, :integer, default: 56
  attr :mini, :boolean, default: false

  def tick_strip(assigns) do
    assigns = assign(assigns, :tones, tick_tones(assigns.values, assigns.count))

    ~H"""
    <div class={[
      "flex items-stretch gap-px",
      @mini && "h-[18px]" ||
        "h-[30px] rounded-md border border-sl-200 bg-canvas-alt px-1.5 py-1 flex flex-row justify-around"
    ]}>
      <span :for={{tone, i} <- Enum.with_index(@tones)} class={["min-w-[3px] max-w-[3px] flex-1 rounded-[3px]", tick_class(tone)]} key={i}>
      </span>
    </div>
    """
  end

  defp tick_class("ok"), do: "bg-seu-500"
  defp tick_class("watch"), do: "bg-unlearn-600"
  defp tick_class("warn"), do: "bg-leak-600"
  defp tick_class("crit"), do: "bg-leak-700"

  @doc "A full probe-path strip: agent→target name, p95 pill, tick-strip. Links to the agent."
  attr :path, :map, required: true

  def probe_strip(assigns) do
    ~H"""
    <.link navigate={~p"/agents/#{@path.serial_id}"} class="flex flex-col gap-1.5 py-2">
      <div class="flex items-baseline gap-2">
        <span class="font-mono text-xs text-sl-700">
          <span class="text-ink">{@path.serial_id}</span>
          <span class="text-sl-400">→</span> {@path.target}
        </span>
        <span class="ml-auto">
          <.pill tone={(rtt_warn?(@path.p95_ns) && "warn") || "ok"}>{fmt_rtt(@path.p95_ns)}</.pill>
        </span>
      </div>
      <.tick_strip values={@path.spark} count={60} />
    </.link>
    """
  end

  defp rtt_warn?(ns) when is_integer(ns), do: ns > 15_000_000
  defp rtt_warn?(_), do: false

  @doc "Triage detail panel for the Alerts inbox. `detail` is `%{event, findings}`."
  attr :detail, :map, required: true

  def alert_detail(assigns) do
    e = assigns.detail.event
    fs = assigns.detail.findings
    trend = fs |> Enum.map(& &1.value) |> Enum.reverse()
    duration = fmt_duration(DateTime.diff(e.closed_at || DateTime.utc_now(), e.opened_at))

    assigns =
      assigns
      |> assign(:event, e)
      |> assign(:findings, fs)
      |> assign(:trend, trend)
      |> assign(:duration, duration)

    ~H"""
    <.card body_class="pad">
      <div class="mb-3 flex items-center gap-2">
        <.sev z={@event.peak_severity} />
        <.event_badge status={@event.display_status} />
      </div>
      <div class="mb-0.5 font-mono text-lg font-semibold text-ink">{@event.target}</div>
      <div class="mb-3 flex flex-wrap items-center gap-1.5 text-xs text-sl-500">
        <.metric_chip metric={@event.metric} /> on
        <.link navigate={~p"/agents/#{@event.serial_id}"} class="font-medium text-encoder-700 hover:underline">
          {@event.serial_id}
        </.link>
      </div>

      <div class="mb-3 rounded-lg border border-leak-600/20 bg-leak-50 p-3">
        <div class="mb-2 font-mono text-[10px] uppercase tracking-wide text-leak-700">
          measured value · last {length(@findings)} samples
        </div>
        <.tick_strip values={@trend} count={max(length(@trend), 1)} />
      </div>

      <div>
        <.kv k="Detectors" mono>{fmt_detectors(@event.detectors)}</.kv>
        <.kv k="Peak severity" mono>{fmt_severity(@event.peak_severity)} σ</.kv>
        <.kv k="Findings" mono>{@event.finding_count}</.kv>
        <.kv k="Opened" mono>{fmt_dt(@event.opened_at)}</.kv>
        <.kv k="Last seen" mono>{fmt_dt(@event.last_seen_at)}</.kv>
        <.kv k="Closed" mono>{(@event.closed_at && fmt_dt(@event.closed_at)) || "—"}</.kv>
        <.kv k="Duration" mono>{@duration}</.kv>
      </div>

      <div class="mt-2 flex gap-2">
        <.link
          navigate={~p"/agents/#{@event.serial_id}"}
          class="inline-flex flex-1 items-center justify-center gap-1.5 rounded-[5px] border border-sl-200 bg-paper px-2.5 py-1.5 text-xs font-medium text-sl-700 hover:bg-canvas-alt"
        >
          <.icon name="hero-cpu-chip" class="size-3.5" /> Device
        </.link>
      </div>
    </.card>
    """
  end

  # ===========================================================================
  # Target selector + task editor (carried over from the previous dashboard)
  # ===========================================================================

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
      <span class="text-xs text-sl-500">{@label}:</span>
      <span :if={@available == []} class="text-xs text-sl-400">no targets yet</span>
      <label :for={t <- @available} class="flex cursor-pointer items-center gap-1">
        <input
          type="checkbox"
          name="targets[]"
          value={t}
          checked={target_checked?(@selected, t)}
          class="size-3 accent-encoder-600"
        />
        <span class="font-mono text-xs text-sl-700">{t}</span>
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
  Submitting inserts them and bumps the agent generation. (Kept from the prior
  dashboard — the new design has no dedicated home for it, so it lives at the
  bottom of the agent page.)
  """
  def task_form(assigns) do
    assigns =
      assign(assigns, :recommendations, Dashboard.recommended_trace_targets(assigns.rows, assigns.tasks))

    ~H"""
    <.card title="Add probe tasks">
      <:actions>
        <span :if={@rows != []} class="text-xs text-sl-500">
          saving inserts the tasks &amp; bumps the agent generation
        </span>
      </:actions>

      <form id="add-tasks-form" phx-change="task_form_change" phx-submit="save_tasks" class="flex flex-col gap-2">
        <p :if={@rows == []} class="text-sm text-sl-600">
          No tasks queued. Add one below to push new probes to this agent.
        </p>

        <div
          :for={row <- @rows}
          id={"task-#{row.ref}"}
          class="flex flex-wrap items-center gap-2 rounded-lg border border-sl-200 bg-canvas-alt/50 px-3 py-2"
        >
          <select name={"tasks[#{row.ref}][type]"} class="select select-bordered select-sm w-40">
            <option :for={{label, value} <- Dashboard.task_types()} value={value} selected={value == row.type}>
              {label}
            </option>
          </select>

          <span class="text-sl-400">→</span>

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

          <label :if={Dashboard.task_has_port?(row.type)} class="flex items-center gap-1 text-xs text-sl-600">
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

          <label :if={Dashboard.task_schedulable?(row.type)} class="flex items-center gap-1 text-xs text-sl-600">
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
          <span :if={not Dashboard.task_schedulable?(row.type)} class="text-xs text-sl-500">
            long-running listener
          </span>

          <button
            type="button"
            phx-click="remove_task_row"
            phx-value-ref={row.ref}
            class="btn btn-ghost btn-sm btn-square ml-auto text-sl-500 hover:text-leak-600"
            aria-label="Remove task"
          >
            <.icon name="hero-x-mark" class="size-4" />
          </button>
        </div>

        <div
          :for={target <- @recommendations}
          class="flex flex-wrap items-center gap-2 rounded-lg border border-dashed border-unlearn-600/40 bg-unlearn-50 px-3 py-2 text-sm"
        >
          <.icon name="hero-light-bulb" class="size-4 shrink-0 text-unlearn-600" />
          <span>Traceroute to <span class="font-mono">{target}</span> has no latency probe.</span>
          <div class="ml-auto flex gap-1">
            <button
              type="button"
              class="btn btn-xs btn-outline"
              phx-click="add_task_row"
              phx-value-type="ping"
              phx-value-target={target}
            >
              + Ping
            </button>
            <button
              type="button"
              class="btn btn-xs btn-outline"
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
    </.card>
    """
  end

  # ===========================================================================
  # Presentation helpers
  # ===========================================================================

  @doc "\"received/sent\" probe-count label, or \"—\" for a missing row."
  def sent_label(nil), do: "—"
  def sent_label(%{received: r, sent: s}), do: "#{r}/#{s}"

  @doc "Look up a per-target ping rollup's avg p95 (ns) from a `target => row` map."
  def ping_p95(map, target) do
    case Map.get(map, target) do
      nil -> nil
      row -> row.avg_p95_ns
    end
  end

  @doc "Pill tone for a latency in ns: warn above 15 ms, else ok."
  def rtt_tone(nil), do: "neutral"
  def rtt_tone(ns) when is_integer(ns) and ns > 15_000_000, do: "warn"
  def rtt_tone(ns) when is_integer(ns), do: "ok"
  def rtt_tone(_), do: "neutral"

  @doc "Pill tone for a loss fraction (0–1): warn above 0.5%, else ok."
  def loss_tone_for(nil), do: "neutral"
  def loss_tone_for(f) when is_float(f) and f > 0.005, do: "warn"
  def loss_tone_for(f) when is_float(f), do: "ok"
  def loss_tone_for(_), do: "neutral"

  @doc "Severity → tone band. nil → nominal."
  def severity_tone(z) when is_number(z) and z >= 4.0, do: "crit"
  def severity_tone(z) when is_number(z) and z >= 3.0, do: "warn"
  def severity_tone(z) when is_number(z) and z >= 1.5, do: "watch"
  def severity_tone(_), do: "nominal"

  @doc "Resample `values` to `count` ticks and tone each relative to the series max."
  def tick_tones([], count), do: List.duplicate("ok", count)

  def tick_tones(values, count) do
    nums = Enum.filter(values, &is_number/1)

    case nums do
      [] ->
        List.duplicate("ok", count)

      _ ->
        max = Enum.max(nums)
        max = if max <= 0, do: 1, else: max

        values
        |> resample(count)
        |> Enum.map(fn v -> ratio_tone((v || 0) / max) end)
    end
  end

  defp ratio_tone(r) when r > 0.9, do: "crit"
  defp ratio_tone(r) when r > 0.75, do: "warn"
  defp ratio_tone(r) when r > 0.5, do: "watch"
  defp ratio_tone(_), do: "ok"

  defp resample(values, count) do
    n = length(values)

    cond do
      n == 0 -> List.duplicate(0, count)
      count <= 1 -> [List.last(values)]
      true ->
        for i <- 0..(count - 1) do
          idx = trunc(i / (count - 1) * (n - 1))
          Enum.at(values, idx, List.last(values))
        end
    end
  end

  @doc "A device is online if its newest measurement is within 2× its report interval."
  def device_online?(device, summary) do
    threshold = max(device.report_interval_seconds * 2, 30)

    case summary |> Enum.map(& &1.last_seen) |> Enum.filter(& &1) do
      [] ->
        false

      stamps ->
        latest = Enum.reduce(stamps, fn a, b -> if DateTime.compare(a, b) == :gt, do: a, else: b end)
        DateTime.diff(DateTime.utc_now(), latest) <= threshold
    end
  end

  def short_uuid(nil), do: ""
  def short_uuid(uuid) when is_binary(uuid), do: String.slice(uuid, 0, 8) <> "…"

  def fmt_dt(%DateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S UTC")
  def fmt_dt(%NaiveDateTime{} = dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")
  def fmt_dt(nil), do: "—"
  def fmt_dt(other), do: to_string(other)

  @doc "Compact 'Ns / Nm Ns / Nh Nm ago'-style duration between two datetimes."
  def fmt_ago(nil), do: "—"

  def fmt_ago(%DateTime{} = dt) do
    secs = DateTime.diff(DateTime.utc_now(), dt)
    fmt_duration(secs) <> " ago"
  end

  def fmt_ago(_), do: "—"

  def fmt_duration(secs) when secs < 60, do: "#{max(secs, 0)}s"

  def fmt_duration(secs) when secs < 3600 do
    "#{div(secs, 60)}m #{rem(secs, 60)}s"
  end

  def fmt_duration(secs), do: "#{div(secs, 3600)}h #{rem(div(secs, 60), 60)}m"

  def fmt_loss(nil), do: "—"
  def fmt_loss(f) when is_float(f), do: fmt_pct(f * 100)
  def fmt_loss(other), do: to_string(other)

  @doc "Format a 0–100 percentage as e.g. \"0.44%\"."
  def fmt_pct(nil), do: "—"
  def fmt_pct(p) when is_number(p), do: fmt_num(p) <> "%"

  @doc "Format a number to 2 decimals as a binary."
  def fmt_num(nil), do: "—"
  def fmt_num(n) when is_number(n), do: :io_lib.format("~.2f", [n / 1]) |> IO.iodata_to_binary()

  def fmt_rtt(nil), do: "—"

  def fmt_rtt(ns) when is_integer(ns) do
    :io_lib.format("~.2f ms", [ns / 1_000_000]) |> IO.iodata_to_binary()
  end

  def fmt_rtt(other), do: to_string(other)

  @doc "Bare ms number (no unit), e.g. for vitals: \"28.83\"."
  def fmt_ms(nil), do: "—"
  def fmt_ms(ns) when is_integer(ns), do: fmt_num(ns / 1_000_000)

  def fmt_severity(nil), do: "—"
  def fmt_severity(s) when is_number(s), do: fmt_num(s)
  def fmt_severity(other), do: to_string(other)

  @doc "Detectors list → uppercase ' · '-joined label."
  def fmt_detectors([]), do: "—"
  def fmt_detectors(list) when is_list(list), do: list |> Enum.join(" · ") |> String.upcase()
  def fmt_detectors(_), do: "—"

  @doc "Truncate JSONB-as-text for inline display."
  def trunc_details(nil), do: ""
  def trunc_details(s) when is_binary(s) and byte_size(s) > 80, do: binary_part(s, 0, 80) <> "…"
  def trunc_details(s) when is_binary(s), do: s

  @doc """
  Collapses a flat, ts-descending list of findings into one collapsible group
  per `{target, port, metric, detector}` stream. Each group carries its own
  `:expanded` flag (from the `expanded` MapSet) and a stable `:key` for toggling.
  """
  def group_findings(findings, expanded) do
    {order, groups} =
      Enum.reduce(findings, {[], %{}}, fn f, {order, groups} ->
        key = {f.target, f.port, f.metric, f.detector}

        case groups do
          %{^key => acc} -> {order, %{groups | key => [f | acc]}}
          _ -> {[key | order], Map.put(groups, key, [f])}
        end
      end)

    order
    |> Enum.reverse()
    |> Enum.map(fn {target, port, metric, detector} = key ->
      fs = Enum.reverse(groups[key])
      latest = hd(fs)
      key_string = finding_group_key(target, port, metric, detector)

      %{
        key: key_string,
        expanded: MapSet.member?(expanded, key_string),
        target: target,
        port: port,
        metric: metric,
        detector: detector,
        count: length(fs),
        latest_ts: latest.ts,
        latest_z: latest.severity,
        peak_severity: peak_finding_severity(fs),
        trend: fs |> Enum.reverse() |> Enum.map(& &1.value),
        findings: fs
      }
    end)
  end

  defp finding_group_key(target, port, metric, detector),
    do: Enum.join([target, port, metric, detector], "|")

  defp peak_finding_severity(findings) do
    case findings |> Enum.map(& &1.severity) |> Enum.filter(&is_number/1) do
      [] -> nil
      severities -> Enum.max(severities)
    end
  end
end
