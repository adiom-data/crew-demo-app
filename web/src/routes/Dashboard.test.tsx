import { create } from "@bufbuild/protobuf";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { partnerClient } from "../api/clients";
import { BillingStatus, PartnerSchema, Status, Tier, type Partner } from "../gen/sample/v1/partner_pb";
import { Dashboard } from "./Dashboard";

vi.mock("../api/clients", () => ({
  partnerClient: { listPartners: vi.fn() },
}));

const createObjectURL = vi.fn(() => "blob:partners");
const revokeObjectURL = vi.fn();
const anchorClick = vi.fn();

function partner(overrides: Partial<Partner>): Partner {
  return create(PartnerSchema, {
    id: "partner-1",
    name: "Acme",
    contactEmail: "ops@acme.com",
    company: "Acme Inc",
    region: "US",
    tier: Tier.PRO,
    status: Status.ACTIVE,
    billingStatus: BillingStatus.CURRENT,
    createdAt: "2026-01-15T00:00:00Z",
    ...overrides,
  });
}

function mockListPartners(partners: Partner[]) {
  vi.mocked(partnerClient.listPartners).mockResolvedValue({
    partners,
    total: partners.length,
    active: partners.filter((p) => p.status === Status.ACTIVE).length,
    pending: partners.filter((p) => p.status === Status.PENDING).length,
  });
}

function renderPage() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>,
  );
}

describe("Dashboard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: revokeObjectURL });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(anchorClick);
  });

  it("renders loaded partners", async () => {
    mockListPartners([
      partner({ id: "partner-1", name: "Acme", status: Status.ACTIVE }),
      partner({ id: "partner-2", name: "Beta", status: Status.PENDING }),
    ]);

    renderPage();

    expect(await screen.findByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(screen.getByText(/showing 2 of 2 partners/i)).toBeInTheDocument();
  });

  it("filters by status", async () => {
    const user = userEvent.setup();
    mockListPartners([
      partner({ id: "partner-1", name: "Acme", status: Status.ACTIVE }),
      partner({ id: "partner-2", name: "Beta", status: Status.PENDING }),
    ]);

    renderPage();
    await screen.findByText("Acme");

    await user.selectOptions(screen.getByRole("combobox", { name: /status/i }), String(Status.PENDING));

    expect(screen.queryByText("Acme")).not.toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(screen.getByText(/showing 1 of 2 partners/i)).toBeInTheDocument();
  });

  it("shows a filter-empty state and disables export", async () => {
    const user = userEvent.setup();
    mockListPartners([partner({ id: "partner-1", name: "Acme", status: Status.ACTIVE })]);

    renderPage();
    await screen.findByText("Acme");

    await user.selectOptions(screen.getByRole("combobox", { name: /status/i }), String(Status.PENDING));

    expect(screen.getByText(/no partners match the pending filter/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /export csv/i })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: /clear filter/i }));

    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /export csv/i })).toBeEnabled();
  });

  it("exports only currently filtered rows", async () => {
    const user = userEvent.setup();
    mockListPartners([
      partner({ id: "partner-1", name: "Acme", status: Status.ACTIVE, contactEmail: "ops@acme.com" }),
      partner({ id: "partner-2", name: "Beta", status: Status.PENDING, contactEmail: "ops@beta.com" }),
    ]);

    renderPage();
    await screen.findByText("Acme");

    await user.selectOptions(screen.getByRole("combobox", { name: /status/i }), String(Status.ACTIVE));
    await user.click(screen.getByRole("button", { name: /export csv/i }));

    await waitFor(() => expect(createObjectURL).toHaveBeenCalled());
    const blob = createObjectURL.mock.calls[0][0] as Blob;
    const csv = await blob.text();

    expect(csv).toContain("Acme");
    expect(csv).toContain("ops@acme.com");
    expect(csv).not.toContain("Beta");
    expect(csv).not.toContain("ops@beta.com");
    expect(anchorClick).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:partners");
  });
});
