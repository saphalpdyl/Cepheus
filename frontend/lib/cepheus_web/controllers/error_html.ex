defmodule CepheusWeb.ErrorHTML do
  @moduledoc """
      Displays error

  """
  use CepheusWeb, :html

  def render(template, _assigns) do
    Phoenix.Controller.status_message_from_template(template)
  end
end
