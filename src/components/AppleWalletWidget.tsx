import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

interface WalletItem {
  id: string;
  type: 'card' | 'pass';
  name: string;
  lastUpdated: string;
}

const WidgetContainer = styled.div`
  background: #ffffff;
  border-radius: 12px;
  padding: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  margin: 16px;
`;

const WidgetHeader = styled.div`
  display: flex;
  align-items: center;
  margin-bottom: 16px;
`;

const WalletIcon = styled.div`
  width: 24px;
  height: 24px;
  margin-right: 8px;
  background-color: #000;
  mask: url('/assets/wallet-icon.svg') no-repeat center;
  -webkit-mask: url('/assets/wallet-icon.svg') no-repeat center;
`;

const Title = styled.h2`
  margin: 0;
  font-size: 18px;
  font-weight: 600;
`;

const ItemsList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
`;

const ItemCard = styled.div`
  padding: 12px;
  background: #f8f8f8;
  border-radius: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const AppleWalletWidget: React.FC = () => {
  const [walletItems, setWalletItems] = useState<WalletItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchWalletItems = async () => {
    try {
      // TODO: Replace with actual API call
      const response = await fetch('/api/wallet/items');
      const data = await response.json();
      setWalletItems(data);
    } catch (error) {
      console.error('Error fetching wallet items:', error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchWalletItems();

    // Set up automatic refresh every 5 minutes
    const refreshInterval = setInterval(fetchWalletItems, 5 * 60 * 1000);

    return () => {
      clearInterval(refreshInterval);
    };
  }, []);

  if (isLoading) {
    return (
      <WidgetContainer>
        <WidgetHeader>
          <WalletIcon />
          <Title>Apple Wallet</Title>
        </WidgetHeader>
        <div>Loading...</div>
      </WidgetContainer>
    );
  }

  return (
    <WidgetContainer>
      <WidgetHeader>
        <WalletIcon />
        <Title>Apple Wallet</Title>
      </WidgetHeader>
      <ItemsList>
        {walletItems.length === 0 ? (
          <div>No cards or passes found</div>
        ) : (
          walletItems.map((item) => (
            <ItemCard key={item.id}>
              <div>
                <div>{item.name}</div>
                <div style={{ fontSize: '12px', color: '#666' }}>
                  {item.type.charAt(0).toUpperCase() + item.type.slice(1)}
                </div>
              </div>
              <div style={{ fontSize: '12px', color: '#666' }}>
                Last updated: {new Date(item.lastUpdated).toLocaleDateString()}
              </div>
            </ItemCard>
          ))
        )}
      </ItemsList>
    </WidgetContainer>
  );
};

export default AppleWalletWidget;