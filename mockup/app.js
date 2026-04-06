const rankingData = [
  {
    rank: 1,
    node: "symbol-manila-01",
    status: "qualified",
    score: 91.8,
    availability: "99.96%",
    finalization: "19.8 / 20",
    sync: "9.8 / 10",
    country: "PH",
    group: "independent-a",
    domain: "example.net",
  },
  {
    rank: 2,
    node: "symbol-tokyo-02",
    status: "qualified",
    score: 90.4,
    availability: "99.92%",
    finalization: "19.2 / 20",
    sync: "9.7 / 10",
    country: "JP",
    group: "independent-b",
    domain: "nodejp.net",
  },
  {
    rank: 3,
    node: "symbol-eu-west-04",
    status: "watch",
    score: 87.3,
    availability: "99.12%",
    finalization: "17.9 / 20",
    sync: "9.4 / 10",
    country: "DE",
    group: "provider-x",
    domain: "provider-x.net",
  },
  {
    rank: 4,
    node: "symbol-sg-01",
    status: "qualified",
    score: 86.9,
    availability: "99.51%",
    finalization: "18.5 / 20",
    sync: "9.3 / 10",
    country: "SG",
    group: "independent-c",
    domain: "sg-node.org",
  },
  {
    rank: 5,
    node: "symbol-us-central-03",
    status: "watch",
    score: 84.1,
    availability: "99.08%",
    finalization: "17.2 / 20",
    sync: "9.1 / 10",
    country: "US",
    group: "provider-x",
    domain: "provider-x.net",
  },
  {
    rank: 6,
    node: "symbol-frankfurt-02",
    status: "qualified",
    score: 83.6,
    availability: "99.71%",
    finalization: "18.0 / 20",
    sync: "9.0 / 10",
    country: "DE",
    group: "independent-d",
    domain: "ops-de.cloud",
  },
];

const rankingBody = document.querySelector("#rankingBody");
const statusFilter = document.querySelector("#statusFilter");
const searchInput = document.querySelector("#searchInput");

function renderRows(items) {
  rankingBody.innerHTML = items
    .map((item) => {
      const badgeClass =
        item.status === "qualified" ? "badge badge--qualified" : "badge badge--watch";
      const badgeText = item.status === "qualified" ? "Qualified" : "Watch";

      return `
        <tr>
          <td>${item.rank}</td>
          <td>
            <strong>${item.node}</strong><br />
            <small>${item.domain}</small>
          </td>
          <td><span class="${badgeClass}">${badgeText}</span></td>
          <td>${item.score.toFixed(1)}</td>
          <td>${item.availability}</td>
          <td>${item.finalization}</td>
          <td>${item.sync}</td>
          <td>${item.country}</td>
          <td>${item.group}</td>
        </tr>
      `;
    })
    .join("");
}

function applyFilters() {
  const statusValue = statusFilter.value;
  const searchValue = searchInput.value.trim().toLowerCase();

  const filtered = rankingData.filter((item) => {
    const statusMatch = statusValue === "all" || item.status === statusValue;
    const searchMatch =
      !searchValue ||
      item.node.toLowerCase().includes(searchValue) ||
      item.domain.toLowerCase().includes(searchValue) ||
      item.country.toLowerCase().includes(searchValue) ||
      item.group.toLowerCase().includes(searchValue);

    return statusMatch && searchMatch;
  });

  renderRows(filtered);
}

statusFilter.addEventListener("change", applyFilters);
searchInput.addEventListener("input", applyFilters);

renderRows(rankingData);
