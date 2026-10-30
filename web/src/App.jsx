import { useEffect, useState } from 'react';

function money(value) {
  return `$${Number(value).toFixed(2)}`;
}

export default function App({ address = '0xabc123' }) {
  const [portfolio, setPortfolio] = useState(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const controller = new AbortController();

    setLoading(true);
    setPortfolio(null);
    setError('');

    async function load() {
      try {
        const response = await fetch(
          `/api/wallets/${encodeURIComponent(address)}/portfolio`,
          { signal: controller.signal }
        );

        let body;
        try {
          body = await response.json();
        } catch {
          throw new Error('The portfolio service returned an invalid response');
        }

        if (!response.ok) {
          throw new Error(body?.error?.message || 'Unable to load portfolio');
        }

        if (!body || !Array.isArray(body.tokens)) {
          throw new Error('The portfolio service returned an invalid response');
        }

        if (!controller.signal.aborted) {
          setPortfolio(body);
        }
      } catch (err) {
        if (!controller.signal.aborted && err.name !== 'AbortError') {
          setError(err.message || 'Unable to load portfolio');
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }

    load();
    return () => controller.abort();
  }, [address]);

  if (loading) {
    return <main aria-busy="true"><p role="status">Loading portfolio...</p></main>;
  }

  if (error) {
    return <main role="alert">{error}</main>;
  }

  if (!portfolio) {
    return <main />;
  }

  if (portfolio.tokens.length === 0) {
    return (
      <main>
        <h1>Wallet Portfolio</h1>
        <p>No tokens found for this wallet.</p>
      </main>
    );
  }

  const hasUnavailablePrices = portfolio.tokens.some((token) => !token.price_available);

  return (
    <main>
      <h1>Wallet Portfolio</h1>
      <p>Total value: {money(portfolio.total_usd)}</p>
      {hasUnavailablePrices && (
        <p role="status">Total excludes tokens whose prices are currently unavailable.</p>
      )}
      <table>
        <thead>
          <tr>
            <th>Token</th>
            <th>Amount</th>
            <th>Price</th>
            <th>Value</th>
          </tr>
        </thead>
        <tbody>
          {portfolio.tokens.map((token) => (
            <tr key={`${token.chain}-${token.symbol}`}>
              <td>{token.symbol}</td>
              <td>{token.amount}</td>
              <td>{token.price_available ? money(token.price_usd) : 'Price unavailable'}</td>
              <td>{token.price_available ? money(token.value_usd) : '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
