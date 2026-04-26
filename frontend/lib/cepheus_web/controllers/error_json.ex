defmodule CepheusWeb.ErrorJSON do
  @moduledoc """
  invoked by endpoint in case of errors on JSON requests.

  See config/config.exs.
  """
  def render(template, _assigns) do
    %{errors: %{detail: Phoenix.Controller.status_message_from_template(template)}}
  end
end
