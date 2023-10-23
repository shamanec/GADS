import rethinkdb from 'rethinkdb'

export async function newConnection() {
    try{
        const connection = await rethinkdb.connect({
            host: process.env.NEXT_RETHINKDB_HOST,
            port: Number(process.env.NEXT_RETHINKDB_PORT),
            db: 'gads',
        })

        return connection
    }catch(error) {
        throw new Error(`Could not connect to RethinkDB, err: ${error}`)
    }
}