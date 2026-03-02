import clsx from 'clsx';

interface SkeletonProps {
    className?: string;
}

function Skeleton({ className }: SkeletonProps) {
    return (
        <div
            className={clsx(
                'animate-pulse rounded-xl bg-apple-gray-200/60',
                className
            )}
        />
    );
}

// Pre-built skeleton layouts for common patterns

function CardSkeleton() {
    return (
        <div className="bg-white rounded-2xl shadow-apple p-6 space-y-4">
            <div className="flex items-center justify-between">
                <Skeleton className="h-5 w-24" />
                <Skeleton className="h-8 w-8 rounded-full" />
            </div>
            <Skeleton className="h-8 w-32" />
            <Skeleton className="h-4 w-20" />
        </div>
    );
}

function StatCardsSkeleton({ count = 4 }: { count?: number }) {
    return (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {Array.from({ length: count }).map((_, i) => (
                <CardSkeleton key={i} />
            ))}
        </div>
    );
}

function TableSkeleton({ rows = 5, columns = 4 }: { rows?: number; columns?: number }) {
    return (
        <div className="bg-white rounded-2xl shadow-apple overflow-hidden">
            {/* Header */}
            <div className="px-6 py-4 border-b border-apple-gray-200 flex gap-4">
                {Array.from({ length: columns }).map((_, i) => (
                    <Skeleton key={i} className="h-4 flex-1" />
                ))}
            </div>
            {/* Rows */}
            {Array.from({ length: rows }).map((_, rowIdx) => (
                <div
                    key={rowIdx}
                    className="px-6 py-4 border-b border-apple-gray-100 flex gap-4 items-center"
                >
                    {Array.from({ length: columns }).map((_, colIdx) => (
                        <Skeleton
                            key={colIdx}
                            className={clsx('h-4 flex-1', colIdx === 0 && 'max-w-[200px]')}
                        />
                    ))}
                </div>
            ))}
        </div>
    );
}

function ChartSkeleton() {
    return (
        <div className="bg-white rounded-2xl shadow-apple p-6 space-y-4">
            <div className="flex items-center justify-between">
                <Skeleton className="h-5 w-32" />
                <div className="flex gap-2">
                    <Skeleton className="h-8 w-16 rounded-lg" />
                    <Skeleton className="h-8 w-16 rounded-lg" />
                </div>
            </div>
            <Skeleton className="h-64 w-full rounded-xl" />
        </div>
    );
}

function PageSkeleton() {
    return (
        <div className="space-y-6 animate-in fade-in duration-300">
            {/* Page header */}
            <div className="flex items-center justify-between">
                <div className="space-y-2">
                    <Skeleton className="h-8 w-48" />
                    <Skeleton className="h-4 w-72" />
                </div>
                <Skeleton className="h-10 w-32 rounded-xl" />
            </div>
            {/* Stat cards */}
            <StatCardsSkeleton />
            {/* Chart */}
            <ChartSkeleton />
            {/* Table */}
            <TableSkeleton />
        </div>
    );
}

export { Skeleton, CardSkeleton, StatCardsSkeleton, TableSkeleton, ChartSkeleton, PageSkeleton };
export default Skeleton;
