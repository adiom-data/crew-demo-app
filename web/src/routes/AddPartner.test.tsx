import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AddPartner } from "./AddPartner";
import { partnerClient } from "../api/clients";

vi.mock("../api/clients", () => ({
  partnerClient: { createPartner: vi.fn() },
}));

function renderPage() {
  return render(
    <MemoryRouter>
      <AddPartner />
    </MemoryRouter>,
  );
}

describe("AddPartner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the form", () => {
    renderPage();
    expect(screen.getByRole("button", { name: /create partner/i })).toBeInTheDocument();
  });

  it("shows a validation error and does not submit when email is missing", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.type(screen.getByLabelText(/name/i), "Acme");
    await user.click(screen.getByRole("button", { name: /create partner/i }));

    expect(await screen.findByText(/contact email is required/i)).toBeInTheDocument();
    expect(partnerClient.createPartner).not.toHaveBeenCalled();
  });
});
