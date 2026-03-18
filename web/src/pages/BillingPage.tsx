import { useState, useEffect } from 'react';
import { 
  CreditCardIcon, 
  ArrowPathIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
  DocumentDuplicateIcon
} from '@heroicons/react/24/outline';
import { plansApi, Order, getApiErrorMessage } from '@/lib/api';
import toast from 'react-hot-toast';

function BillingPage() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchOrders();
  }, []);

  const fetchOrders = async () => {
    try {
      setLoading(true);
      const response = await plansApi.getOrders();
      setOrders(response.data);
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'Failed to fetch billing history'));
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'paid':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
            <CheckCircleIcon className="w-3 h-3 mr-1" />
            Paid
          </span>
        );
      case 'pending':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-orange-100 text-orange-800">
            <ClockIcon className="w-3 h-3 mr-1 animate-pulse" />
            Pending
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800">
            <XCircleIcon className="w-3 h-3 mr-1" />
            Failed
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-apple-gray-800">
            {status}
          </span>
        );
    }
  };

  const copyOrderNo = (orderNo: string) => {
    navigator.clipboard.writeText(orderNo);
    toast.success('Order number copied');
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-semibold text-apple-gray-900">Billing History</h1>
          <p className="text-apple-gray-500">View and manage your recent payments and invoices</p>
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <ArrowPathIcon className="w-8 h-8 text-apple-blue animate-spin" />
        </div>
      ) : orders.length === 0 ? (
        <div className="bg-white rounded-apple border border-apple-gray-200 p-12 text-center">
          <CreditCardIcon className="w-12 h-12 text-apple-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-apple-gray-900">No Billing History</h3>
          <p className="text-apple-gray-500 max-w-sm mx-auto mt-2">
            You haven't made any payments yet. Subscribe to a plan to get more features.
          </p>
        </div>
      ) : (
        <div className="bg-white rounded-apple border border-apple-gray-200 overflow-hidden shadow-sm">
          <table className="min-w-full divide-y divide-apple-gray-200">
            <thead className="bg-apple-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">Order Info</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">Amount</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-apple-gray-500 uppercase tracking-wider">Date</th>
                <th className="px-6 py-3 text-right text-xs font-medium text-apple-gray-500 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-apple-gray-200">
              {orders.map((order) => (
                <tr key={order.id} className="hover:bg-apple-gray-50 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap">
                    <div className="flex items-center">
                      <div>
                        <div className="text-sm font-medium text-apple-gray-900 flex items-center">
                          {order.order_no}
                          <button 
                            onClick={() => copyOrderNo(order.order_no)}
                            className="ml-2 text-apple-gray-400 hover:text-apple-blue"
                          >
                            <DocumentDuplicateIcon className="w-4 h-4" />
                          </button>
                        </div>
                        <div className="text-xs text-apple-gray-500">{order.payment_method}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    {getStatusBadge(order.status)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-semibold text-apple-gray-900">
                    ${order.amount.toFixed(2)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-apple-gray-500">
                    {new Date(order.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button className="text-apple-blue hover:text-blue-700">Receipt</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export default BillingPage;
